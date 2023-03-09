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

package spanmanager

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/awslabs/soci-snapshotter/ztoc/compression"
)

type spanState int

var errInvalidSpanStateTransition = errors.New("invalid span state transition")

const (
	// A span is in Unrequested state when it's not requested from remote.
	unrequested spanState = iota
	// A span is in Requested state when it's requested from remote but its content hasn't been returned.
	requested
	// A span is in Fetched state when its content is fetched from remote and compressed data is cached.
	fetched
	// A span is in Uncompressed state when it's uncompressed and its uncompressed content is cached.
	uncompressed
)

const (
	// Default number of tries fetching data from remote and verifying the digest.
	defaultSpanVerificationFailureRetries = 3
)

// map of valid span transtions: current state -> valid new states.
// stateTransitionMap is kept minimum so we won't change state by accident.
// We should keep it documented when each transition will happen.
var stateTransitionMap = map[spanState][]spanState{
	unrequested: {
		// when span starts being fetched; it makes other goroutines aware of this
		requested,
	},
	requested: {
		// when a span fetch fails; change back to unrequested so other goroutines can request again
		unrequested,
		// when bg-fetcher fetches and caches compressed span
		fetched,
		// when span data request comes; span is fetched, uncompressed and cached
		uncompressed,
	},
	fetched: {
		// when span data request comes and span is fetched by bg-fetcher; compressed span is available in cache
		uncompressed,
	},
}

type span struct {
	id                compression.SpanID
	startCompOffset   compression.Offset
	endCompOffset     compression.Offset
	startUncompOffset compression.Offset
	endUncompOffset   compression.Offset
	state             atomic.Value
	mu                sync.Mutex
}

func (s *span) checkState(expected spanState) bool {
	state := s.state.Load().(spanState)
	return state == expected
}

func (s *span) setState(state spanState) error {
	err := s.validateStateTransition(state)
	if err != nil {
		return err
	}
	s.state.Store(state)
	return nil
}

func (s *span) validateStateTransition(newState spanState) error {
	state := s.state.Load().(spanState)
	for _, s := range stateTransitionMap[state] {
		if newState == s {
			return nil
		}
	}
	return fmt.Errorf("%w: %v -> %v", errInvalidSpanStateTransition, state, newState)
}
