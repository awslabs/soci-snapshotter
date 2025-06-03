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

package internal

import (
	"context"
	"strings"

	clicontext "github.com/awslabs/soci-snapshotter/cmd/internal/context"
	"github.com/awslabs/soci-snapshotter/soci/store"
)

// ContentStoreOptions builds a list of content store options from a CLI context.
func ContentStoreOptions(ctx context.Context) ([]store.Option, error) {
	contentStore, err := clicontext.GetValue[string](ctx, "content-store")
	if err != nil {
		return []store.Option{}, err
	}
	address, err := clicontext.GetValue[string](ctx, "address")
	if err != nil {
		return []store.Option{}, err
	}
	return []store.Option{
		store.WithType(store.ContentStoreType(contentStore)),
		store.WithContainerdAddress(strings.TrimPrefix(address, "unix://")),
	}, nil
}
