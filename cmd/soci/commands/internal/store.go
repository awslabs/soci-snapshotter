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
	"github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/urfave/cli/v2"
)

// ContentStoreOptions builds a list of content store options from a CLI context.
func ContentStoreOptions(context *cli.Context) []store.Option {
	return []store.Option{
		store.WithType(store.ContentStoreType(context.String("content-store"))),
		store.WithContainerdAddress(context.String("address")),
	}
}
