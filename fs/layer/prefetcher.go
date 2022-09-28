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

package layer

import (
	"errors"
	"io"

	spanmanager "github.com/awslabs/soci-snapshotter/fs/span-manager"
	sociindex "github.com/awslabs/soci-snapshotter/soci/index"
)

type prefetcher struct {
	r           *io.SectionReader // reader for prefetching the layer
	spanManager *spanmanager.SpanManager
}

func newPrefetcher(r *io.SectionReader, spanManager *spanmanager.SpanManager) *prefetcher {
	p := prefetcher{
		r:           r,
		spanManager: spanManager,
	}
	return &p
}

func (p *prefetcher) prefetch() error {
	var spanID sociindex.SpanID
	for {
		err := p.spanManager.ResolveSpan(spanID, p.r)
		if errors.Is(err, spanmanager.ErrExceedMaxSpan) {
			break
		}
		if err != nil {
			return err
		}
		spanID++
	}
	return nil
}
