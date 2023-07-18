#!/bin/bash

# Read the input JSON file
input_file="../benchmark/performanceTest/output/results.json"

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

  mkdir -p ../pre-processed-results
  # Define the output JSON file name
  local output_file="../pre-processed-results/${test_name}.json"

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
commit=$(jq -r '.commit' "$input_file")
tests=$(jq -r '.benchmarkTests | length' "$input_file")

# Loop through each test and extract the required data
for ((i = 0; i < tests; i++)); do
  testName=$(jq -r --argjson i $i '.benchmarkTests[$i].testName' "$input_file")

  # Lazy Task Stats
  lazyTaskPct90=$(jq -r --argjson i $i '.benchmarkTests[$i].lazyTaskStats.pct90' "$input_file")

  # Local Task Stats
  localTaskPct90=$(jq -r --argjson i $i '.benchmarkTests[$i].localTaskStats.pct90' "$input_file")

  pullTaskPct90=$(jq -r --argjson i $i '.benchmarkTests[$i].pullStats.pct90' "$input_file")

  # Create JSON file for each testName
  create_json_file "$testName" "$lazyTaskPct90" "$localTaskPct90" "$pullTaskPct90"
done
