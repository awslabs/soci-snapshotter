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

package framework

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"testing"

	"github.com/containerd/containerd/log"
	"github.com/montanaflynn/stats"
)

var (
	resultFilename             = "results.json"
	resultFilePerm fs.FileMode = 0644
)

type BenchmarkFramework struct {
	OutputDir string                `json:"-"`
	CommitID  string                `json:"commit"`
	Drivers   []BenchmarkTestDriver `json:"benchmarkTests"`
}

type BenchmarkTestStats struct {
	BenchmarkTimes []float64 `json:"BenchmarkTimes"`
	StdDev         float64   `json:"stdDev"`
	Mean           float64   `json:"mean"`
	Min            float64   `json:"min"`
	Pct25          float64   `json:"pct25"`
	Pct50          float64   `json:"pct50"`
	Pct75          float64   `json:"pct75"`
	Pct90          float64   `json:"pct90"`
	Max            float64   `json:"max"`
}

type BenchmarkTestDriver struct {
	TestName       string             `json:"testName"`
	NumberOfTests  int                `json:"numberOfTests"`
	BeforeFunction func()             `json:"-"`
	TestFunction   func(*testing.B)   `json:"-"`
	AfterFunction  func() error       `json:"-"`
	TestsRun       int                `json:"-"`
	FullRunStats   BenchmarkTestStats `json:"fullRunStats"`
	PullStats      BenchmarkTestStats `json:"pullStats"`
	LazyTaskStats  BenchmarkTestStats `json:"lazyTaskStats"`
	LocalTaskStats BenchmarkTestStats `json:"localTaskStats"`
}

func (frame *BenchmarkFramework) Run(ctx context.Context) {
	testing.Init()
	flag.Set("test.benchtime", "1x")
	flag.Parse()
	for i := 0; i < len(frame.Drivers); i++ {
		testDriver := &frame.Drivers[i]
		fmt.Printf("Running tests for %s\n", testDriver.TestName)
		if testDriver.BeforeFunction != nil {
			testDriver.BeforeFunction()
		}
		for j := 0; j < testDriver.NumberOfTests; j++ {
			log.G(ctx).WithField("test_name", testDriver.TestName).Infof("TestStart for " + testDriver.TestName + "_" + strconv.Itoa(j+1))
			fmt.Printf("Running test %d of %d\n", j+1, testDriver.NumberOfTests)
			res := testing.Benchmark(testDriver.TestFunction)
			testDriver.FullRunStats.BenchmarkTimes = append(testDriver.FullRunStats.BenchmarkTimes, res.T.Seconds())
			testDriver.PullStats.BenchmarkTimes = append(testDriver.PullStats.BenchmarkTimes, res.Extra["pullDuration"]/1000)
			testDriver.LazyTaskStats.BenchmarkTimes = append(testDriver.LazyTaskStats.BenchmarkTimes, res.Extra["lazyTaskDuration"]/1000)
			testDriver.LocalTaskStats.BenchmarkTimes = append(testDriver.LocalTaskStats.BenchmarkTimes, res.Extra["localTaskStats"]/1000)
		}
		testDriver.calculateStats()
		if testDriver.AfterFunction != nil {
			err := testDriver.AfterFunction()
			if err != nil {
				fmt.Printf("After function error: %v\n", err)
			}
		}
	}

	print("should We add timeout here for testing?")
	json, err := json.MarshalIndent(frame, "", " ")
	if err != nil {
		fmt.Printf("JSON Marshalling Error: %v\n", err)
	}
	err = os.MkdirAll(frame.OutputDir, resultFilePerm)
	if err != nil {
		fmt.Printf("Failed to Create Output Dir: %v\n", err)
	}
	resultFileLoc := frame.OutputDir + "/" + resultFilename
	err = os.WriteFile(resultFileLoc, json, resultFilePerm)
	if err != nil {
		fmt.Printf("WriteFile Error: %v\n", err)
	}
}

func (driver *BenchmarkTestDriver) calculateStats() {
	driver.FullRunStats.calculateTestStat()
	driver.PullStats.calculateTestStat()
	driver.LazyTaskStats.calculateTestStat()
	driver.LocalTaskStats.calculateTestStat()
}

func (testStats *BenchmarkTestStats) calculateTestStat() {
	var err error
	testStats.StdDev, err = stats.StandardDeviation(testStats.BenchmarkTimes)
	if err != nil {
		fmt.Printf("Error Calculating Std Dev: %v\n", err)
		testStats.StdDev = -1
	}
	testStats.Mean, err = stats.Mean(testStats.BenchmarkTimes)
	if err != nil {
		fmt.Printf("Error Calculating Mean: %v\n", err)
		testStats.Mean = -1
	}
	testStats.Min, err = stats.Min(testStats.BenchmarkTimes)
	if err != nil {
		fmt.Printf("Error Calculating Min: %v\n", err)
		testStats.Min = -1
	}
	testStats.Pct25, err = stats.Percentile(testStats.BenchmarkTimes, 25)
	if err != nil {
		fmt.Printf("Error Calculating 25th Pct: %v\n", err)
		testStats.Pct25 = -1
	}
	testStats.Pct50, err = stats.Percentile(testStats.BenchmarkTimes, 50)
	if err != nil {
		fmt.Printf("Error Calculating 50th Pct: %v\n", err)
		testStats.Pct50 = -1
	}
	testStats.Pct75, err = stats.Percentile(testStats.BenchmarkTimes, 75)
	if err != nil {
		fmt.Printf("Error Calculating 75th Pct: %v\n", err)
		testStats.Pct75 = -1
	}
	testStats.Pct90, err = stats.Percentile(testStats.BenchmarkTimes, 90)
	if err != nil {
		fmt.Printf("Error Calculating 90th Pct: %v\n", err)
		testStats.Pct90 = -1
	}
	testStats.Max, err = stats.Max(testStats.BenchmarkTimes)
	if err != nil {
		fmt.Printf("Error Calculating Max: %v\n", err)
		testStats.Max = -1
	}
}
