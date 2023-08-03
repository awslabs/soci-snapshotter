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

package http

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"time"

	"github.com/awslabs/soci-snapshotter/config"
	"github.com/awslabs/soci-snapshotter/version"
	"github.com/containerd/containerd/log"
	rhttp "github.com/hashicorp/go-retryablehttp"
	"github.com/sirupsen/logrus"
)

var (
	UserAgent = fmt.Sprintf("soci-snapshotter/%s", version.Version)
)

// NewRetryableClient creates a go http.Client which will automatically
// retry on non-fatal errors
func NewRetryableClient(config config.RetryableHTTPClientConfig) *http.Client {
	rhttpClient := rhttp.NewClient()
	// Don't log every request
	rhttpClient.Logger = nil

	// set retry config
	rhttpClient.RetryMax = config.MaxRetries
	rhttpClient.RetryWaitMin = time.Duration(config.MinWaitMsec) * time.Millisecond
	rhttpClient.RetryWaitMax = time.Duration(config.MaxWaitMsec) * time.Millisecond
	rhttpClient.Backoff = BackoffStrategy
	rhttpClient.CheckRetry = RetryStrategy
	rhttpClient.HTTPClient.Timeout = time.Duration(config.RequestTimeoutMsec) * time.Millisecond

	// set timeouts
	innerTransport := rhttpClient.HTTPClient.Transport
	if t, ok := innerTransport.(*http.Transport); ok {
		t.DialContext = (&net.Dialer{
			Timeout: time.Duration(config.DialTimeoutMsec) * time.Millisecond,
		}).DialContext
		t.ResponseHeaderTimeout = time.Duration(config.ResponseHeaderTimeoutMsec) * time.Millisecond
	}

	return rhttpClient.StandardClient()
}

// Jitter returns a number in the range duration to duration+(duration/divisor)-1, inclusive
func Jitter(duration time.Duration, divisor int64) time.Duration {
	return time.Duration(rand.Int63n(int64(duration)/divisor) + int64(duration))
}

// BackoffStrategy extends retryablehttp's DefaultBackoff to add a random jitter to avoid
// overwhelming the repository when it comes back online
// DefaultBackoff either tries to parse the 'Retry-After' header of the response; or, it uses an
// exponential backoff 2 ^ numAttempts, limited by max
func BackoffStrategy(min, max time.Duration, attemptNum int, resp *http.Response) time.Duration {
	delayTime := rhttp.DefaultBackoff(min, max, attemptNum, resp)
	return Jitter(delayTime, 8)
}

// RetryStrategy extends retryablehttp's DefaultRetryPolicy to log the error and response when retrying
// DefaultRetryPolicy retries whenever err is non-nil (except for some url errors) or if returned
// status code is 429 or 5xx (except 501)
func RetryStrategy(ctx context.Context, resp *http.Response, err error) (bool, error) {
	retry, err2 := rhttp.DefaultRetryPolicy(ctx, resp, err)
	if retry {
		log.G(ctx).WithFields(logrus.Fields{
			"error":    err,
			"response": resp,
		}).Debugf("retrying request")
	}
	return retry, err2
}
