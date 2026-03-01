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

set -eu -o pipefail

cleanup() {
    rm test_output.txt
    rm failed_tests.txt
}

if [ "$#" -ne 1 ]; then
    echo "Expected 1 parameter, got $#."
    echo "Usage: $0 [num_retries]"
    exit 1
fi

if [ "$1" -lt 0 ]; then
    echo "Invalid number of retries specified (must be non-negative integer)."
    exit 1
fi

# Run tests with systemd by default
if [ -z ${SKIP_SYSTEMD_TESTS+""} ]; then
    SKIP_SYSTEMD_TESTS=0
fi

echo "Running integration tests with max $1 retries"
# Truncate files if they exist
echo -n "" > test_output.txt
echo -n "" > failed_tests.txt

for i in $(seq 0 "$1"); do
    echo -n "" > test_output.txt

    run_failed_tests_cmd=""
    if [ "$(cat failed_tests.txt | wc -l)" -ne 0 ]; then
        # go test uses regex to figure out which tests to run, so we may sometimes
        # run duplicate tests that we don't need to rerun.
        failed_tests=$(cat failed_tests.txt | paste -sd "|")
        run_failed_tests_cmd="-run='$failed_tests'"
    fi
    echo "SKIP_SYSTEMD_TESTS=$SKIP_SYSTEMD_TESTS GO_TEST_FLAGS=$run_failed_tests_cmd make integration-with-coverage"
    # Append || true so that we don't fail out if the testing suite fails
    (SKIP_SYSTEMD_TESTS=$SKIP_SYSTEMD_TESTS GO_TEST_FLAGS=$run_failed_tests_cmd make integration-with-coverage | tee test_output.txt) || true
    
    # $3 is the name of the test in standard go test output
    (cat test_output.txt | grep "FAIL: Test" | grep -v "/" | awk '{print $3}' > failed_tests.txt) || true 
    if [ "$(cat failed_tests.txt | wc -l)" -eq 0 ]; then
        # Ensure that build/setup did not fail
        if ! cat test_output.txt | grep -q "PASS: Test"; then
            echo "Integration testing failed for non-testing suite reasons (probably a build or setup failure)."
            exit 1
        fi
        echo "Integration tests passed in $i retries"
        cleanup
        exit 0
    fi

    printf '\n\n\n\n################\nThe following integration tests failed:\n\n'
    cat failed_tests.txt
    printf '################\n\n\n\n'
done

echo "Integration tests reached max retry limit of $1."
cleanup
exit 1
