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

package context

import (
	"context"
	"fmt"
)

const (
	RootKey = "root"
)

func GetValue[T any](ctx context.Context, key string) (T, error) {
	value := ctx.Value(key)
	if value == nil {
		var zero T
		return zero, fmt.Errorf("key %q not found in context", key)
	}
	val, ok := value.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("value for key %q is not of type %T", key, zero)
	}
	return val, nil
}
