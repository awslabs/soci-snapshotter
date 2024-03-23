#!/usr/bin/env bash

#   Copyright The containerd Authors.

#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at

#       http://www.apache.org/licenses/LICENSE-2.0

#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.

set -eux -o pipefail

# the very first auto-commit doesn't have a DCO and the first real commit has a slightly different format. Exclude those when doing the check.
# We erreneously allowed a non-signed commit to be pushed to main.
# This is a temporary fix, and the commit hash should be changed to HEAD~20 once it is no longer an issue.
"$(go env GOPATH)/bin/git-validation" -run DCO -range 0a9fdda7b507b164d8cfa50c0a51367e9f0e2379..HEAD
