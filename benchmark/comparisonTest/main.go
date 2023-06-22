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
	"testing"

	"github.com/awslabs/soci-snapshotter/benchmark"
	"github.com/awslabs/soci-snapshotter/benchmark/framework"
)

var (
	outputDir = "../comparisonTest/output"
)

func main() {

	var numberOfTests int
	var configCsv string
	var showCom bool
	var parseFileAccessPatterns bool

	flag.BoolVar(&parseFileAccessPatterns, "parseFileAccess", false, "Parse fuse file access patterns.")
	flag.BoolVar(&showCom, "show-commit", false, "tag the commit hash to the benchmark results")
	flag.IntVar(&numberOfTests, "count", 5, "Describes the number of runs a benchmarker should run. Default: 5")
	flag.StringVar(&configCsv, "f", "../imageList.csv", "Path to a csv file describing image details in this order ['Name','Image ref', 'Ready line', 'manifest ref'].")

	flag.Parse()

	var commit string

	if showCom {
		commit, _ = getCommitHash()
	} else {
		commit = "N/A"
	}

	imageList, err := benchmark.GetImageListFromCsv(configCsv)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to read csv file %s with error:%v\n", configCsv, err)
		panic(errMsg)
	}

	err = os.Mkdir(outputDir, 0755)
	if err != nil && !os.IsExist(err) {
		panic(err)
	}

	logFile, err := os.OpenFile(outputDir+"/benchmark_log", os.O_RDWR|os.O_CREATE, 0664)
	if err != nil {
		panic(err)
	}
	defer logFile.Close()
	ctx, cancelCtx := framework.GetTestContext(logFile)
	defer cancelCtx()

	var drivers []framework.BenchmarkTestDriver
	for _, image := range imageList {
		shortName := image.ShortName
		imageRef := image.ImageRef
		sociIndexManifestRef := image.SociIndexManifestRef
		readyLine := image.ReadyLine
		drivers = append(drivers, framework.BenchmarkTestDriver{
			TestName:      "OverlayFSFull" + shortName,
			NumberOfTests: numberOfTests,
			TestFunction: func(b *testing.B) {
				benchmark.OverlayFSFullRun(ctx, b, imageRef, readyLine, "OverlayFSFull"+shortName)
			},
		})
		drivers = append(drivers, framework.BenchmarkTestDriver{
			TestName:      "SociFull" + shortName,
			NumberOfTests: numberOfTests,
			TestFunction: func(b *testing.B) {
				benchmark.SociFullRun(ctx, b, imageRef, sociIndexManifestRef, readyLine, "SociFull"+shortName)
			},
		})
	}

	benchmarks := framework.BenchmarkFramework{
		OutputDir: outputDir,
		CommitID:  commit,
		Drivers:   drivers,
	}
	benchmarks.Run(ctx)
}

func getCommitHash() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}
