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

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v3"
)

const (
	PrefetchFilesFlag     = "prefetch-file"
	PrefetchFilesJSONFlag = "prefetch-files-json"
)

func PrefetchFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringSliceFlag{
			Name:  PrefetchFilesFlag,
			Usage: "File paths to prefetch. Can be specified multiple times or as multiple arguments. These files will be included in the SOCI index metadata for faster access. Example: --prefetch-file /app/config.json --prefetch-file /app/static/main.css",
		},
		&cli.StringFlag{
			Name:  PrefetchFilesJSONFlag,
			Usage: "Path to a JSON file containing a list of file paths to prefetch. The JSON file should contain an array of strings. Example: --prefetch-files-json '/path/to/prefetch.json'",
		},
	}
}

func ParsePrefetchFiles(cmd *cli.Command) ([]string, error) {
	var allPrefetchFiles []string

	prefetchFiles := cmd.StringSlice(PrefetchFilesFlag)
	allPrefetchFiles = append(allPrefetchFiles, trimAndFilterFiles(prefetchFiles)...)

	jsonFilePath := cmd.String(PrefetchFilesJSONFlag)
	if jsonFilePath != "" {
		jsonFiles, err := loadPrefetchFilesFromJSON(jsonFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load prefetch files from JSON: %w", err)
		}
		allPrefetchFiles = append(allPrefetchFiles, jsonFiles...)
	}

	return allPrefetchFiles, nil
}

func loadPrefetchFilesFromJSON(filePath string) ([]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON file: %w", err)
	}

	var files []string
	if err := json.Unmarshal(data, &files); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return trimAndFilterFiles(files), nil
}

func trimAndFilterFiles(files []string) []string {
	var result []string
	for _, file := range files {
		file = strings.TrimSpace(file)
		if file != "" {
			result = append(result, file)
		}
	}
	return result
}
