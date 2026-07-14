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
	"sync"
	"time"

	commonmetrics "github.com/awslabs/soci-snapshotter/fs/metrics/common"
	"github.com/containerd/log"
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

	// All span managers are appended to the work queue and picked up in Run().
	// If a span manager is still able to fetch, it is re-appended.
	//
	// The queue is an unbounded, mutex-guarded slice rather than a fixed-size
	// channel: Add is called on the layer resolve (Mount critical) path, so it
	// must never block; and dropping entries would permanently skip background
	// fetching for those layers. Queued entries are small (a Resolver pointer
	// per layer), so unbounded growth is bounded in practice by the number of
	// concurrently-mounted layers. maxQueueSize is kept as an observability
	// threshold: exceeding it logs a warning but does not block or drop.
	workQueueMu sync.Mutex
	workQueue   []Resolver

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
	bf.closeChan = make(chan struct{})
	bf.pauseChan = make(chan struct{}, 1)

	if bf.bfPauser == nil {
		bf.bfPauser = defaultPauser{}
	}

	return bf, nil
}

// Add a new Resolver to be background fetched from.
// Appends the resolver to the work queue, which is drained by Run().
//
// Add never blocks and never drops: it is called on the layer resolve
// (Mount critical) path, where blocking on a full queue would stall layer
// mounts (previously observed as pulls stalling for a full silence period),
// and dropping would permanently skip background fetching for the layer.
func (bf *BackgroundFetcher) Add(resolver Resolver) {
	bf.workQueueMu.Lock()
	bf.workQueue = append(bf.workQueue, resolver)
	n := len(bf.workQueue)
	bf.workQueueMu.Unlock()
	if bf.maxQueueSize > 0 && n > bf.maxQueueSize {
		log.L.WithField("queueSize", n).WithField("maxQueueSize", bf.maxQueueSize).
			Debug("background fetch work queue exceeded max_queue_size (non-fatal; queue is unbounded)")
	}
}

// pop removes and returns the next Resolver from the work queue, or nil if
// the queue is empty.
func (bf *BackgroundFetcher) pop() Resolver {
	bf.workQueueMu.Lock()
	defer bf.workQueueMu.Unlock()
	if len(bf.workQueue) == 0 {
		return nil
	}
	lr := bf.workQueue[0]
	bf.workQueue = bf.workQueue[1:]
	return lr
}

func (bf *BackgroundFetcher) queueSize() int {
	bf.workQueueMu.Lock()
	defer bf.workQueueMu.Unlock()
	return len(bf.workQueue)
}

func (bf *BackgroundFetcher) Close() error {
	bf.closeChan <- struct{}{}
	return nil
}

// Pause sends a signal to pause the background fetcher for silencePeriod on the next iteration.
// The signal is idempotent (pending signals are coalesced into a single pause),
// so this never blocks even if a signal is already pending.
func (bf *BackgroundFetcher) Pause() {
	select {
	case bf.pauseChan <- struct{}{}:
	default:
		// A pause signal is already pending; coalesce.
	}
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

		if lr := bf.pop(); lr != nil {
			if lr.Closed() {
				continue
			}
			go func() {
				more, err := lr.Resolve(ctx)
				if more {
					bf.Add(lr)
				} else if err != nil {
					log.G(ctx).WithError(err).Warn("error trying to resolve layer, removing it from the queue")
				}
			}()
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
			commonmetrics.AddImageOperationCount(commonmetrics.BackgroundFetchWorkQueueSize, "", int32(bf.queueSize()))
		}
	}
}
