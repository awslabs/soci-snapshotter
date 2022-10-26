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
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"testing"

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

type BenchmarkTestDriver struct {
	TestName       string           `json:"testName"`
	NumberOfTests  int              `json:"numberOfTests"`
	BeforeFunction func()           `json:"-"`
	TestFunction   func(*testing.B) `json:"-"`
	AfterFunction  func()           `json:"-"`
	TestsRun       int              `json:"-"`
	TestTimes      []float64        `json:"testTimes"`
	StdDev         float64          `json:"stdDev"`
	Mean           float64          `json:"mean"`
	Min            float64          `json:"min"`
	Pct25          float64          `json:"pct25"`
	Pct50          float64          `json:"pct50"`
	Pct75          float64          `json:"pct75"`
	Pct90          float64          `json:"pct90"`
	Max            float64          `json:"max"`
}

func (frame *BenchmarkFramework) Run() {
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
			fmt.Printf("Running test %d of %d\n", j+1, testDriver.NumberOfTests)
			res := testing.Benchmark(testDriver.TestFunction)
			testDriver.TestTimes = append(testDriver.TestTimes, res.T.Seconds())
		}
		testDriver.calculateStats()
		if testDriver.AfterFunction != nil {
			testDriver.AfterFunction()
		}
	}

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
	var err error
	driver.StdDev, err = stats.StandardDeviation(driver.TestTimes)
	if err != nil {
		fmt.Printf("Error Calculating Std Dev: %v\n", err)
		driver.StdDev = -1
	}
	driver.Mean, err = stats.Mean(driver.TestTimes)
	if err != nil {
		fmt.Printf("Error Calculating Mean: %v\n", err)
		driver.Mean = -1
	}
	driver.Min, err = stats.Min(driver.TestTimes)
	if err != nil {
		fmt.Printf("Error Calculating Min: %v\n", err)
		driver.Min = -1
	}
	driver.Pct25, err = stats.Percentile(driver.TestTimes, 25)
	if err != nil {
		fmt.Printf("Error Calculating 25th Pct: %v\n", err)
		driver.Pct25 = -1
	}
	driver.Pct50, err = stats.Percentile(driver.TestTimes, 50)
	if err != nil {
		fmt.Printf("Error Calculating 50th Pct: %v\n", err)
		driver.Pct50 = -1
	}
	driver.Pct75, err = stats.Percentile(driver.TestTimes, 75)
	if err != nil {
		fmt.Printf("Error Calculating 75th Pct: %v\n", err)
		driver.Pct75 = -1
	}
	driver.Pct90, err = stats.Percentile(driver.TestTimes, 90)
	if err != nil {
		fmt.Printf("Error Calculating 90th Pct: %v\n", err)
		driver.Pct90 = -1
	}
	driver.Max, err = stats.Max(driver.TestTimes)
	if err != nil {
		fmt.Printf("Error Calculating Max: %v\n", err)
		driver.Max = -1
	}
}
