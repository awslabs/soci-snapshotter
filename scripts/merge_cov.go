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

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/tools/cover"
)

func main() {
	var unitDir, integrationDir, outFile string
	flag.StringVar(&unitDir, "unit", "", "Directory containing unit test coverage data")
	flag.StringVar(&integrationDir, "integration", "", "Directory containing integration test coverage data")
	flag.StringVar(&outFile, "out", "coverage.out", "Output file for total coverage")
	flag.Parse()

	if unitDir == "" || integrationDir == "" {
		fmt.Println("Both unit and integration directories must be specified")
		os.Exit(1)
	}

	// Find all coverage files
	unitFiles, err := findCoverageFiles(unitDir)
	if err != nil {
		fmt.Printf("Error finding unit test coverage files: %v\n", err)
		os.Exit(1)
	}

	integrationFiles, err := findCoverageFiles(integrationDir)
	if err != nil {
		fmt.Printf("Error finding integration test coverage files: %v\n", err)
		os.Exit(1)
	}

	// Merge coverage profiles
	profiles, err := mergeCoverProfiles(append(unitFiles, integrationFiles...))
	if err != nil {
		fmt.Printf("Error merging coverage profiles: %v\n", err)
		os.Exit(1)
	}

	// Write total profile to output file
	if err := writeCoverProfile(profiles, outFile); err != nil {
		fmt.Printf("Error writing total coverage profile: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully total coverage data to %s\n", outFile)
}

func findCoverageFiles(dir string) ([]string, error) {
	var files []string

	// First try to find .out files directly
	outFiles, err := filepath.Glob(filepath.Join(dir, "*.out"))
	if err != nil {
		return nil, err
	}
	files = append(files, outFiles...)

	// If no .out files found, try to convert from covdata format
	if len(files) == 0 {
		// Create a temporary file for the coverage data
		tmpFile := filepath.Join(dir, "coverage.out")
		if err := exec.Command("go", "tool", "covdata", "textfmt", "-i="+dir, "-o="+tmpFile); err == nil {
			files = append(files, tmpFile)
		}
	}

	return files, nil
}

func mergeCoverProfiles(files []string) ([]*cover.Profile, error) {
	var total []*cover.Profile
	profileMap := make(map[string]*cover.Profile)

	for _, file := range files {
		profiles, err := cover.ParseProfiles(file)
		if err != nil {
			return nil, fmt.Errorf("error parsing %s: %v", file, err)
		}

		for _, p := range profiles {
			if existing, ok := profileMap[p.FileName]; ok {
				// Merge blocks for the same file
				for i, block := range p.Blocks {
					if i < len(existing.Blocks) {
						// If the block exists in both profiles, add the counts
						existing.Blocks[i].Count += block.Count
					} else {
						// If the block only exists in the new profile, append it
						existing.Blocks = append(existing.Blocks, block)
					}
				}
			} else {
				// New file, add to map
				profileMap[p.FileName] = p
			}
		}
	}

	// Convert map to slice
	for _, profile := range profileMap {
		total = append(total, profile)
	}

	return total, nil
}

func writeCoverProfile(profiles []*cover.Profile, outFile string) error {
	f, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "mode: set\n")
	for _, profile := range profiles {
		for _, block := range profile.Blocks {
			count := block.Count
			if count > 0 {
				count = 1 // Convert to "set" mode
			}
			fmt.Fprintf(f, "%s:%d.%d,%d.%d %d %d\n",
				profile.FileName, block.StartLine, block.StartCol, block.EndLine, block.EndCol, block.NumStmt, count)
		}
	}

	return nil
}
