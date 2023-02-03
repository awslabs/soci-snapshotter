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

package backgroundfetcher

import (
	"context"
	"fmt"
	"time"

	commonmetrics "github.com/awslabs/soci-snapshotter/fs/metrics/common"
	"github.com/containerd/containerd/log"
	"golang.org/x/time/rate"
)

type Option func(*BackgroundFetcher) error

func WithSilencePeriod(period time.Duration) Option {
	return func(bf *BackgroundFetcher) error {
		bf.silencePeriod = period
		return nil
	}
}

func WithFetchPeriod(period time.Duration) Option {
	return func(bf *BackgroundFetcher) error {
		bf.fetchPeriod = period
		return nil
	}
}

func WithMaxQueueSize(size int) Option {
	return func(bf *BackgroundFetcher) error {
		bf.maxQueueSize = size
		return nil
	}
}

func WithEmitMetricPeriod(period time.Duration) Option {
	return func(bf *BackgroundFetcher) error {
		bf.emitMetricPeriod = period
		return nil
	}
}

// An interface for a type to "pause" the background fetcher.
// Useful for mocking in unit tests.
type pauser interface {
	pause(time.Duration)
}

type defaultPauser struct{}

func (p defaultPauser) pause(d time.Duration) {
	time.Sleep(d)
}

// A backgroundFetcher is responsible for fetching spans from layers
// in the background.
type BackgroundFetcher struct {
	silencePeriod    time.Duration
	fetchPeriod      time.Duration
	maxQueueSize     int
	emitMetricPeriod time.Duration

	rateLimiter *rate.Limiter

	bfPauser pauser

	// All span managers are added to the channel and picked up in Run().
	// If a span manager is still able to fetch, it is reinserted into the chanel.
	workQueue chan Resolver
	closeChan chan struct{}
	pauseChan chan struct{}
}

func NewBackgroundFetcher(opts ...Option) (*BackgroundFetcher, error) {
	bf := new(BackgroundFetcher)
	for _, o := range opts {
		if err := o(bf); err != nil {
			return nil, err
		}
	}
	// Create a rate-limiter that will fetch every bf.fetchPeriod
	// with a burst capacity of 1 (i.e., it will never invoke more than 1 bg-fetch
	// within bf.fetchPeriod)
	bf.rateLimiter = rate.NewLimiter(rate.Every(bf.fetchPeriod), 1)
	bf.workQueue = make(chan Resolver, bf.maxQueueSize)
	bf.closeChan = make(chan struct{})
	bf.pauseChan = make(chan struct{})

	if bf.bfPauser == nil {
		bf.bfPauser = defaultPauser{}
	}

	return bf, nil
}

// Add a new Resolver to be background fetched from.
// Sends the resolver through the channel, which will be received in the Run() method.
func (bf *BackgroundFetcher) Add(resolver Resolver) {
	bf.workQueue <- resolver
}

func (bf *BackgroundFetcher) Close() error {
	bf.closeChan <- struct{}{}
	return nil
}

// Sends a signal to pause the background fetcher for silencePeriod on the next iteration.
func (bf *BackgroundFetcher) Pause() {
	bf.pauseChan <- struct{}{}
}

func (bf *BackgroundFetcher) pause(ctx context.Context) {
	needPause := false
loop:
	for {
		select {
		// A new image has been mounted. Need to pause the background fetcher
		case <-bf.pauseChan:
			needPause = true
		default:
			break loop
		}
	}
	if needPause {
		log.G(ctx).WithField("silencePeriod", bf.silencePeriod).Debug("new image mounted, pausing the background fetcher for silence period")
		bf.bfPauser.pause(bf.silencePeriod)
	}
}

func (bf *BackgroundFetcher) Run(ctx context.Context) error {
	ticker := time.NewTicker(bf.emitMetricPeriod)
	go bf.emitWorkQueueMetric(ctx, ticker)

	for {
		// Pause the background fetcher if necessary.
		bf.pause(ctx)

		select {
		case <-bf.closeChan:
			ticker.Stop()
			return nil
		case <-ctx.Done():
			ticker.Stop()
			return nil
		default:
		}

		select {
		case lr := <-bf.workQueue:
			if lr.Closed() {
				continue
			}
			go func() {
				more, err := lr.Resolve(ctx)
				if more {
					bf.workQueue <- lr
				} else if err != nil {
					log.G(ctx).WithError(err).Warn("error trying to resolve layer, removing it from the queue")
				}
			}()
		default:
		}

		if err := bf.rateLimiter.Wait(ctx); err != nil {
			return fmt.Errorf("background fetch: error while waiting for rate limiter: %w", err)
		}
	}
}

func (bf *BackgroundFetcher) emitWorkQueueMetric(ctx context.Context, ticker *time.Ticker) {
	for {
		select {
		case <-bf.closeChan:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			// background fetcher is at the snapshotter's fs level, so no image digest as key
			commonmetrics.AddImageOperationCount(commonmetrics.BackgroundFetchWorkQueueSize, "", int32(len(bf.workQueue)))
		}
	}
}
