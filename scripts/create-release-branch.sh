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

# A script to create a release branch on origin from the given commit.
#
# Usage: bash create-release-branch.sh [-a|--assert] [-b|--base] [-d|--dry-run] <MAJOR_MINOR_VERSION>

set -eux -o pipefail

ASSERT=false
BASE_COMMIT=""
DRYRUN=false

while [[ $# -gt 0 ]]; do
  case $1 in
    --assert|-a)
      ASSERT=true
      shift # past argument
      ;;
    --base|-b)
      shift # past argument
      BASE_COMMIT=$1
      shift # past value
      ;;
    --dry-run|-d)
      DRYRUN=true
      shift # past argument
      ;;
    --*|-*)
      echo "Unknown option $1"
      exit 1
      ;;
    *)
      VERSION=$1
      shift # past argument
      ;;
  esac
done

sanitize_input() {
  # Strip 'v' prefix from input if present.
  VERSION=${VERSION/v/}
  [[ $VERSION =~ ^[0-9]+\.[0-9]+$ ]] || (echo "Error: version does not match expected <major>.<minor> format" && exit 1)

  if [ -n "$BASE_COMMIT" ]; then
    [[ $BASE_COMMIT =~ ^[0-9a-fA-F]{7,40}$ ]] || (echo "Error: base commit does not match expected short|full format" && exit 1)
    FOUND=$(git log --pretty=format:"%H" | grep "$BASE_COMMIT")
    [ -n "$FOUND" ] || (echo "Error: base commit not found in history" && exit 1)
  fi
}

assert_create_branch() {
  local current_branch
  current_branch=$(git rev-parse --abbrev-ref HEAD)
  [ "$current_branch" = "release/${VERSION}" ] || (echo "Error: incorrect branch, expected: release/${VERSION}, actual: $current_branch" && exit 1)

  local base_commit
  base_commit=$(git show -s --format="%H")
  [ "$base_commit" = "$BASE_COMMIT" ] || (echo "Error: incorrect base commit, expected: $BASE_COMMIT, actual: $base_commit" && exit 1)
}

sanitize_input

git checkout -b "release/${VERSION}" "${BASE_COMMIT}"

[ $ASSERT = true ] && assert_create_branch

PUSH_CMD="git push origin release/${VERSION}"
if [ $DRYRUN = true ]; then
  # Dry-run mode is not able to use git push --dry-run as it still requires
  # write permissions to the remote repository. The intent is to run dry-run mode
  # in pull request workflows with a reduced permission set to mitigate risk for pwn requests.
  # Alternatively assert the push command is correct.
  [[ $PUSH_CMD == "git push origin release/${VERSION}" ]] || (echo "Error: expected: 'git push origin release/${VERSION}', actual: '$PUSH_CMD'" && exit 1)
else
  $PUSH_CMD
fi
