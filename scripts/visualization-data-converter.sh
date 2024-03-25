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

set -eux -o pipefail

if [ $# -ne 2 ]; then
    echo "Usage: $0 <path_to_results.json> <path_to_output_dir.json>"
    exit 1
fi

# Read the input JSON file
input_file="$1"
output_dir="$2"

# Check if the input file exists
if [ ! -f "$input_file" ]; then
  echo "Error: Input file '$input_file' not found."
  exit 1
fi

# Function to create JSON file for each testName
create_json_file() {
  local test_name="$1"
  local lazy_task_value="$2"
  local local_task_value="$3"
  local pull_task_value="$4"

#   mkdir -p ../pre-processed-results
  # Define the output JSON file name
  local output_file="${output_dir}/${test_name}.json"

  # Create the JSON content
  local json_content='[{
    "name": "'"$test_name"'-lazyTaskDuration",
    "unit": "Seconds",
    "value": '"$lazy_task_value"',
    "extra": "P90"
  },
  {
    "name": "'"$test_name"'-localTaskDuration",
    "unit": "Seconds",
    "value": '"$local_task_value"',
    "extra": "P90"
  },
  {
    "name": "'"$test_name"'-pullTaskDuration",
    "unit": "Seconds",
    "value": '"$pull_task_value"',
    "extra": "P90"
  }]'

  # Save the JSON content to the output file
  echo "$json_content" > "$output_file"
}

# Parse the JSON using jq
tests=$(jq -r '.benchmarkTests | length' "$input_file")

# Loop through each test and extract the required data
for ((i = 0; i < tests; i++)); do
  testName=$(jq -r --argjson i "$i" '.benchmarkTests[$i].testName' "$input_file")

  # Lazy Task Stats
  lazyTaskPct90=$(jq -r --argjson i "$i" '.benchmarkTests[$i].lazyTaskStats.pct90' "$input_file")

  # Local Task Stats
  localTaskPct90=$(jq -r --argjson i "$i" '.benchmarkTests[$i].localTaskStats.pct90' "$input_file")

  pullTaskPct90=$(jq -r --argjson i "$i" '.benchmarkTests[$i].pullStats.pct90' "$input_file")

  # Create JSON file for each testName
  create_json_file "$testName" "$lazyTaskPct90" "$localTaskPct90" "$pullTaskPct90"
done
