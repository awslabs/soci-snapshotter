#!/bin/bash

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
    local test_name="$3"
    local stat_name="$4"

    # Calculate 110% of the past value
    local threshold=$(awk -v past="$past_value" 'BEGIN { print past * 1.1 }')

    echo "Test '$test_name' - Past $stat_name P90 value: $past_value"
    echo "Test '$test_name' - Current $stat_name P90 value: $current_value"
    echo "Test '$test_name' - Threshold: $threshold"
    
    # Compare the current value with the threshold
    if (( $(awk 'BEGIN {print ("'"$current_value"'" > "'"$threshold"'")}') )); then
        echo "ERROR: Test '$test_name' - Current P90 value ($current_value) exceeds 110% of past $stat_name P90 value ($past_value)"
        return 1
    fi

    return 0
}

# Loop through each object in past.json and compare P90 values with current.json for all statistics
compare_p90_values() {
    local past_json="$1"
    local current_json="$2"

    local test_names=$(echo "$past_json" | jq -r '.benchmarkTests[].testName')

    for test_name in $test_names; do
        for stat_name in "fullRunStats" "pullStats" "lazyTaskStats" "localTaskStats"; do
            local past_p90=$(echo "$past_json" | jq -r --arg test "$test_name" '.benchmarkTests[] | select(.testName == $test) | .'"$stat_name"'.pct90')
            local current_p90=$(echo "$current_json" | jq -r --arg test "$test_name" '.benchmarkTests[] | select(.testName == $test) | .'"$stat_name"'.pct90')

            compare_stat_p90 "$past_p90" "$current_p90" "$test_name" "$stat_name" || return 1
        done
    done

    return 0
}

compare_p90_values "$past_data" "$current_data"

# Check the return status and display appropriate message
if [ $? -eq 0 ]; then
    echo "Comparison successful. All P90 values are within the acceptable range."
    exit 0
else
    echo "Comparison failed. Current P90 values exceed 110% of the corresponding past values."
    exit 1
fi
