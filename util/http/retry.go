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
	"math/rand"
	"net"
	"net/http"
	"time"

	"github.com/awslabs/soci-snapshotter/config"
	"github.com/containerd/containerd/log"
	rhttp "github.com/hashicorp/go-retryablehttp"
	"github.com/sirupsen/logrus"
)

const (
	// DefaultDialTimeoutMsec is the default number of milliseconds before timeout while connecting to a remote endpoint. See `TimeoutConfig.DialTimeout`.
	DefaultDialTimeoutMsec = 3000
	// DefaultResponseHeaderTimeoutMsec is the default number of milliseconds before timeout while waiting for response header from a remote endpoint. See `TimeoutConfig.ResponseHeaderTimeout`.
	DefaultResponseHeaderTimeoutMsec = 3000
	// DefaultRequestTimeoutMsec is the default number of milliseconds that the entire request can take before timeout. See `TimeoutConfig.RequestTimeout`.
	DefaultRequestTimeoutMsec = 30_000

	// defaults based on a target total retry time of at least 5s. 30*((2^8)-1)>5000

	// DefaultMaxRetries is the default number of retries that a retryable request will make. See `RetryConfig.MaxRetries`.
	DefaultMaxRetries = 8
	// DefaultMinWaitMsec is the default minimum number of milliseconds between attempts. See `RetryConfig.MinWait`.
	DefaultMinWaitMsec = 30
	// DefaultMaxWaitMsec is the default maxmimum number of millisends between attempts. See `RetryConfig.MaxWait`.
	DefaultMaxWaitMsec = 300_000
)

// NewRetryableClientConfig creates a new config with default values.
// Users of `NewRetryableClient` should use this method to get a new
// config and then overwrite values if desired.
func NewRetryableClientConfig() config.RetryableClientConfig {
	return config.RetryableClientConfig{
		TimeoutConfig: config.TimeoutConfig{
			DialTimeout:           DefaultDialTimeoutMsec * time.Millisecond,
			ResponseHeaderTimeout: DefaultResponseHeaderTimeoutMsec * time.Millisecond,
			RequestTimeout:        DefaultRequestTimeoutMsec * time.Millisecond,
		},
		RetryConfig: config.RetryConfig{
			MaxRetries: DefaultMaxRetries,
			MinWait:    DefaultMinWaitMsec * time.Millisecond,
			MaxWait:    DefaultMaxWaitMsec * time.Millisecond,
		},
	}
}

// NewRetryableClient creates a go http.Client which will automatically
// retry on non-fatal errors
func NewRetryableClient(config config.RetryableClientConfig) *http.Client {
	rhttpClient := rhttp.NewClient()
	// Don't log every request
	rhttpClient.Logger = nil

	// set retry config
	rhttpClient.RetryMax = config.MaxRetries
	rhttpClient.RetryWaitMin = config.MinWait
	rhttpClient.RetryWaitMax = config.MaxWait
	rhttpClient.Backoff = BackoffStrategy
	rhttpClient.CheckRetry = RetryStrategy
	rhttpClient.HTTPClient.Timeout = config.RequestTimeout

	// set timeouts
	innerTransport := rhttpClient.HTTPClient.Transport
	if t, ok := innerTransport.(*http.Transport); ok {
		t.DialContext = (&net.Dialer{
			Timeout: config.DialTimeout,
		}).DialContext
		t.ResponseHeaderTimeout = config.ResponseHeaderTimeout
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
