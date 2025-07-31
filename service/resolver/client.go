/*
   Copyright The Soci Snapshotter Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package resolver

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/awslabs/soci-snapshotter/config"
	socihttp "github.com/awslabs/soci-snapshotter/internal/http"
	"github.com/awslabs/soci-snapshotter/version"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/log"
	rhttp "github.com/hashicorp/go-retryablehttp"
	"github.com/sirupsen/logrus"
)

var userAgent = fmt.Sprintf("soci-snapshotter/%s", version.Version)

// globalHeaders returns a global http.Header that should be attached
// to all requests.
func globalHeaders() http.Header {
	header := http.Header{}
	header.Set("User-Agent", userAgent)
	return header
}

// newAuthClient returns a new AuthClient.
func newAuthClient(retryClient *rhttp.Client, header http.Header, creds func(string) (string, string, error)) (*socihttp.AuthClient, error) {

	authorizer := docker.NewDockerAuthorizer(
		docker.WithAuthClient(retryClient.StandardClient()),
		docker.WithAuthCreds(creds), docker.WithAuthHeader(header),
	)

	authClientOpts := []socihttp.AuthClientOpt{
		socihttp.WithRetryableClient(retryClient),
		socihttp.WithAuthPolicy(shouldAuthenticate), socihttp.WithHeader(header),
		socihttp.WithAuthRequestCtxFunc(newContextWithScope),
	}

	authClient, err := socihttp.NewAuthClient(newDockerAuthHandler(authorizer), authClientOpts...)
	if err != nil {
		return nil, err
	}
	return authClient, nil
}

// newRetryableClientFromConfig creates a retryable HTTP client which will automatically
// retry on non-fatal errors given a RetryableHTTPClientConfig.
func newRetryableClientFromConfig(config config.RetryableHTTPClientConfig) *rhttp.Client {
	rhttpClient := rhttp.NewClient()
	// Don't log every request
	rhttpClient.Logger = nil

	// set retry config
	rhttpClient.RetryMax = config.MaxRetries
	rhttpClient.RetryWaitMin = time.Duration(config.MinWaitMsec) * time.Millisecond
	rhttpClient.RetryWaitMax = time.Duration(config.MaxWaitMsec) * time.Millisecond
	rhttpClient.Backoff = backoffStrategy
	rhttpClient.CheckRetry = retryStrategy
	rhttpClient.ErrorHandler = handleHTTPError

	// set timeouts
	rhttpClient.HTTPClient.Timeout = time.Duration(config.RequestTimeoutMsec) * time.Millisecond
	innerTransport := rhttpClient.HTTPClient.Transport
	if t, ok := innerTransport.(*http.Transport); ok {
		t.DialContext = (&net.Dialer{
			Timeout: time.Duration(config.DialTimeoutMsec) * time.Millisecond,
		}).DialContext
		t.ResponseHeaderTimeout = time.Duration(config.ResponseHeaderTimeoutMsec) * time.Millisecond
	}

	return rhttpClient
}

// CloneRetryableClient returns a clone of a given retryable client with the same set
// of retry policies and a new concrete http.Client.
func CloneRetryableClient(retryClient *rhttp.Client) *rhttp.Client {
	newRetryClient := rhttp.NewClient()

	newRetryClient.Logger = retryClient.Logger

	newRetryClient.RetryMax = retryClient.RetryMax
	newRetryClient.RetryWaitMin = retryClient.RetryWaitMin
	newRetryClient.RetryWaitMax = retryClient.RetryWaitMax
	newRetryClient.CheckRetry = retryClient.CheckRetry
	newRetryClient.Backoff = retryClient.Backoff
	newRetryClient.ErrorHandler = retryClient.ErrorHandler

	return newRetryClient
}

// jitter returns a number in the range duration to duration+(duration/divisor)-1, inclusive
func jitter(duration time.Duration, divisor int64) time.Duration {
	return time.Duration(rand.Int64N(int64(duration)/divisor) + int64(duration))
}

// backoffStrategy extends retryablehttp's DefaultBackoff to add a random jitter to avoid
// overwhelming the repository when it comes back online
// DefaultBackoff either tries to parse the 'Retry-After' header of the response; or, it uses an
// exponential backoff 2 ^ numAttempts, limited by max
func backoffStrategy(minDuration, maxDuration time.Duration, attemptNum int, resp *http.Response) time.Duration {
	delayTime := rhttp.DefaultBackoff(minDuration, maxDuration, attemptNum, resp)
	return jitter(delayTime, 8)
}

// retryStrategy extends retryablehttp's DefaultRetryPolicy to log the error and response when retrying
// DefaultRetryPolicy retries whenever err is non-nil (except for some url errors) or if returned
// status code is 429 or 5xx (except 501)
func retryStrategy(ctx context.Context, resp *http.Response, err error) (bool, error) {
	retry, err2 := rhttp.DefaultRetryPolicy(ctx, resp, err)
	if retry {
		log.G(ctx).WithFields(logrus.Fields{
			"error":    socihttp.RedactHTTPQueryValuesFromError(err),
			"response": resp,
		}).Debugf("retrying request")
	}
	return retry, socihttp.RedactHTTPQueryValuesFromError(err2)
}

// handleHTTPError implements retryablehttp client's ErrorHandler to ensure returned errors
// have HTTP query values redacted to prevent leaking sensitive information like encoded credentials or tokens.
func handleHTTPError(resp *http.Response, err error, attempts int) (*http.Response, error) {
	var (
		method = "unknown"
		url    = "unknown"
	)
	if resp != nil {
		socihttp.Drain(resp.Body)
		if resp.Request != nil {
			method = resp.Request.Method
			if resp.Request.URL != nil {
				socihttp.RedactHTTPQueryValuesFromURL(resp.Request.URL)
				url = resp.Request.URL.Redacted()
			}
		}
	}
	if err == nil {
		return nil, fmt.Errorf("%s \"%s\": giving up request after %d attempt(s)", method, url, attempts)
	}

	err = socihttp.RedactHTTPQueryValuesFromError(err)
	return nil, fmt.Errorf("%s \"%s\": giving up request after %d attempt(s): %w", method, url, attempts, err)
}

const (
	ecrTokenExpiredResponseMessage = "Your authorization token has expired. Reauthenticate and try again."
	s3TokenExpiredResponseCode     = "ExpiredToken"
)

type dockerAuthHandler struct {
	authorizer docker.Authorizer
}

// newDockerAuthHandler implements the AuthHandler interface, using
// a docker.Authorizer to handle authentication.
func newDockerAuthHandler(authorizer docker.Authorizer) socihttp.AuthHandler {
	return &dockerAuthHandler{
		authorizer: authorizer,
	}
}

// HandleChallenge calls the underlying docker.Authorizer's AddResponses method.
func (d *dockerAuthHandler) HandleChallenge(ctx context.Context, resp *http.Response) error {
	log.G(ctx).Infof("Received status code: %v. Authorizing...", resp.Status)
	// Prepare authorization for the target host using docker.Authorizer.
	// The authorizer should auto-refresh any expired tokens.
	return d.authorizer.AddResponses(ctx, []*http.Response{resp})

}

// AuthorizeRequest calls the underlying docker.Authorizer's Authorize method.
func (d *dockerAuthHandler) AuthorizeRequest(ctx context.Context, req *http.Request) (*http.Request, error) {
	err := d.authorizer.Authorize(ctx, req)
	return req, err
}

// shouldAuthenticate takes a HTTP response from a registry and determines whether or not
// it warrants authentication.
func shouldAuthenticate(resp *http.Response) bool {
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return true
	case http.StatusForbidden:
		/*
			Although in most cases 403 responses represent authorization issues that generally
			cannot be resolved by re-authentication, some registries like ECR, will return a 403 on
			credential expiration.
			See: https://docs.aws.amazon.com/AmazonECR/latest/userguide/common-errors-docker.html#error-403)

			In the case of ECR, the response body is structured according to the error format defined in the
			Docker v2 API spec. See: https://distribution.github.io/distribution/spec/api/#errors).
			We will attempt to decode the response body as a `docker.Errors`. If it can be decoded,
			we will ensure that the `Message` represents token expiration.
		*/

		// Since we drain the response body, we will copy it to a
		// buffer and re-assign it so that callers can still read
		// from it.
		body, err := io.ReadAll(resp.Body)
		defer func() {
			resp.Body.Close()
			resp.Body = io.NopCloser(bytes.NewReader(body))
		}()

		if err != nil {
			return false
		}

		var errs docker.Errors
		if err = json.Unmarshal(body, &errs); err != nil {
			return false
		}
		for _, e := range errs {
			if err, ok := e.(docker.Error); ok {
				if err.Message == ecrTokenExpiredResponseMessage {
					// ECR's 403 doesn't return a Www-Authenticate and so doesn't trigger the
					// basic re-authentication in containerd's docker authorizer.
					// Ideally ECR would return the Www-Authenticate for expired tokens,
					// but until then we'll have to use this workaround.
					if resp.Header == nil {
						resp.Header = map[string][]string{}
					}

					authenticateHeader := http.CanonicalHeaderKey("Www-Authenticate")
					if _, exists := resp.Header[authenticateHeader]; !exists {
						realm := ""
						if resp.Request != nil {
							realm = "https://" + resp.Request.URL.Host + "/"
						}
						service := "ecr.amazonaws.com"

						authHeaderContent := fmt.Sprintf("Basic realm=\"%s\",service=\"%s\"", realm, service)
						resp.Header[authenticateHeader] = []string{authHeaderContent}
					}

					return true
				}
			}
		}
	case http.StatusBadRequest:
		/*
			S3 returns a 400 on token expiry with an XML encoded response body.
			See: https://docs.aws.amazon.com/AmazonS3/latest/API/ErrorResponses.html#ErrorCodeList

			We will decode the response body and ensure the `Code` represents token expiration.
			If it does, we will normalize the response status (eg: convert it to a standard 401 Unauthorized).
			The pre-signed S3 URL will need to be refreshed by the underlying blob fetcher.

		*/

		if resp.Header.Get("Content-Type") == "application/xml" {
			var s3Error struct {
				XMLName   xml.Name `xml:"Error"`
				Code      string   `xml:"Code"`
				Message   string   `xml:"Message"`
				Resource  string   `xml:"Resource"`
				RequestID string   `xml:"RequestId"`
			}
			body, err := io.ReadAll(resp.Body)
			defer func() {
				resp.Body.Close()
				resp.Body = io.NopCloser(bytes.NewReader(body))
			}()
			if err != nil {
				return false
			}
			if err = xml.Unmarshal(body, &s3Error); err != nil {
				return false
			}
			if s3Error.Code == s3TokenExpiredResponseCode {
				resp.Status = "401 Unauthorized"
				resp.StatusCode = http.StatusUnauthorized
			}
			return false
		}
	default:
	}

	return false
}

// newContextWithScope returns a new context that contains the registry auth scope in the
// original context.
func newContextWithScope(origReqCtx context.Context) context.Context {
	scope := docker.GetTokenScopes(origReqCtx, []string{})
	return docker.WithScope(context.Background(), strings.Join(scope, ""))
}
