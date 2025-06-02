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

import "github.com/urfave/cli/v2"

const (
	ExistingIndexFlagName = "existing-index"
	Warn                  = "warn"
	Skip                  = "skip"
	Allow                 = "allow"
)

var SupportedExistingIndexOptions = []string{Warn, Skip, Allow}

var ExistingIndexFlag = &cli.StringFlag{
	Name:  ExistingIndexFlagName,
	Value: Warn,
	Usage: `Configure how to handle existing SOCI artifacts in remote when pushing indices
			warn  - print warning message to stdout but push index anyway
			skip  - skip pushing the index
			allow - push the index regardless
	`,
}

// SupportedArg checks if a value is present within a given slice
func SupportedArg[K comparable](v K, list []K) bool {
	for _, o := range list {
		if v == o {
			return true
		}
	}
	return false
}
