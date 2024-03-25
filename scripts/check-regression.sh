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

# Check if two arguments are provided (paths to past.json and current.json)
if [ $# -ne 2 ]; then
    echo "Usage: $0 <path_to_past.json> <path_to_current.json>"
    exit 1
fi

# Extract the file paths from command-line arguments
past_json_path="$1"
current_json_path="$2"

# Read the contents of past.json and current.json into variables
past_data=$(cat "$past_json_path")
current_data=$(cat "$current_json_path")

# Function to compare P90 values for a given statistic
compare_stat_p90() {
    local past_value="$1"
    local current_value="$2"
    local stat_name="$3"

    # Calculate 150% of the past value
    local threshold
    threshold=$(calculate_threshold "$past_value")

    # Compare the current value with the threshold
    if (( $(echo "$current_value > $threshold" |bc -l) )); then
        echo "ERROR: $stat_name - Current P90 value ($current_value) exceeds the 110% threshold ($threshold) of the past P90 value ($past_value)"
        return 1
    fi

    return 0
}

calculate_threshold() {
    local past_value="$1"
    awk -v past="$past_value" 'BEGIN { print past * 1.1 }'
}

# calculate the p90 ignoring the first result because we generally see an outlier in the first result
calculate_p90_after_skip() {
    local times_array="$1"

    local num_entries times sorted_times index
    num_entries=$(echo "$times_array" | jq 'length')
    times=$(echo "$times_array" | jq -r '.[1:] | .[]')
    sorted_times=$(echo "$times" | tr '\n' ' ' | xargs -n1 | sort -g)
    index=$((num_entries * 90 / 100))

    local p90
    p90=$(echo "$sorted_times" | sed -n "${index}p")
    echo "$p90"
}

# Loop through each object in past.json and compare P90 values with current.json for all statistics
compare_p90_values() {
    local past_json="$1"
    local current_json="$2"

    local test_names
    test_names=$(echo "$past_json" | jq -r '.benchmarkTests[].testName')

    # Use a flag to indicate if any regression has been detected
    local regression_detected=0

    for test_name in $test_names; do
        echo "Checking for regression in '$test_name'"
        for stat_name in "fullRunStats" "pullStats" "lazyTaskStats" "localTaskStats"; do
            local past_array past_p90 current_array current_p90
            past_array=$(echo "$past_json" | jq -r --arg test "$test_name" '.benchmarkTests[] | select(.testName == $test) | .'"$stat_name"'.BenchmarkTimes')
            past_p90=$(calculate_p90_after_skip "$past_array")
            current_array=$(echo "$current_json" | jq -r --arg test "$test_name" '.benchmarkTests[] | select(.testName == $test) | .'"$stat_name"'.BenchmarkTimes')
            current_p90=$(calculate_p90_after_skip "$current_array")

            # Call the compare_stat_p90 function
            compare_stat_p90 "$past_p90" "$current_p90" "$stat_name" || regression_detected=1
        done
    done

    # Check if any regression has been detected and return the appropriate exit code
    return $regression_detected
}

# Call compare_p90_values and store the exit code in a variable
compare_p90_values "$past_data" "$current_data"
exit_code=$?

# Check the return status and display appropriate message
if [ $exit_code -eq 0 ]; then
    echo "Comparison successful. No regressions detected, all P90 values are within the acceptable range."
else
    echo "Comparison failed. Regression detected."
fi

# Set the final exit code to indicate if any regression occurred
exit $exit_code
