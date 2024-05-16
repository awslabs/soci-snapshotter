#!/usr/bin/env bash

#   Copyright The Soci Snapshotter Authors.

#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at

#       http://www.apache.org/licenses/LICENSE-2.0

#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.

# A script to assert version in the getting started guide was updated
# correctly by GitHub Actions workflow.
#
# Usage: bash update-getting-started-guide-version.sh [-a|--assert] [-v|--verbose] <RELEASE_TAG>

set -eux -o pipefail

tag=$1

ASSERT=false
VERBOSE=false

while [[ $# -gt 0 ]]; do
  case $1 in
    --assert|-a)
      ASSERT=true
      shift # past argument
      ;;
    --verbose|-v)
      VERBOSE=true
      shift # past argument
      ;;
    --*|-*)
      echo "Unknown option $1"
      exit 1
      ;;
    *)
      tag=$1
      shift # past argument
      ;;
  esac
done

# Strip 'v' prefix from tag if not already stripped.
VERSION=${tag/v/} 

assert_diff() {
  local diff_output
  # Disable warning for A && B || C is not if-then-else; C may run when A is true.
  # Branch B contains exit, so C will not run when A and B branches fail.
  # This is intended to have the assertion fail if the diff is empty.
  # shellcheck disable=SC2015
  diff_output=$(git diff --exit-code) && {
    echo "Error: no changes made; expected getting started version to be updated to \"${VERSION}\"" && exit 1
  } || {
    if [[ "${diff_output}" == *"+version=\"${VERSION}\""* ]]; then
      echo "Diff looks good!"
    else
      echo "Error: release version not set properly" && exit 1
    fi
  }
}

sed -i -E "s/version=\"([0-9]+\.){2}[0-9]+\"/version=\"${VERSION}\"/" docs/getting-started.md

[ $VERBOSE = true ] && git diff
[ $ASSERT = true ] && assert_diff
