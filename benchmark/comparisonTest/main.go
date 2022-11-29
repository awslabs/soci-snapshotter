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
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/awslabs/soci-snapshotter/benchmark"
	"github.com/awslabs/soci-snapshotter/benchmark/framework"
)

var (
	outputDir = "./output"
)

func main() {
	commit := os.Args[1]
	configCsv := os.Args[2]
	numberOfTests, err := strconv.Atoi(os.Args[3])
	if err != nil {
		errMsg := fmt.Sprintf("Failed to parse number of test %s with error:%v\n", os.Args[3], err)
		panic(errMsg)
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
