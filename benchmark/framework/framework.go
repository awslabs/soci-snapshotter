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
	TestStats      BenchmarkTestStats `json:"testStats"`
	PullStats      BenchmarkTestStats `json:"pullStats"`
	RunTask1Stats  BenchmarkTestStats `json:"runTask1Stats"`
	RunTask2Stats  BenchmarkTestStats `json:"runTaskStats_WithoutOverhead"`
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
			testDriver.TestStats.BenchmarkTimes = append(testDriver.TestStats.BenchmarkTimes, res.T.Seconds())
			testDriver.PullStats.BenchmarkTimes = append(testDriver.PullStats.BenchmarkTimes, res.Extra["pullDuration"]/1000)
			testDriver.RunTask1Stats.BenchmarkTimes = append(testDriver.RunTask1Stats.BenchmarkTimes, res.Extra["runTask1Duration"]/1000)
			testDriver.RunTask2Stats.BenchmarkTimes = append(testDriver.RunTask2Stats.BenchmarkTimes, res.Extra["runTask2Duration"]/1000)
		}
		testDriver.calculateStats()
		if testDriver.AfterFunction != nil {
			err := testDriver.AfterFunction()
			if err != nil {
				fmt.Printf("After function error: %v\n", err)
			}
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
	driver.TestStats.StdDev, err = stats.StandardDeviation(driver.TestStats.BenchmarkTimes)
	if err != nil {
		fmt.Printf("Error Calculating Std Dev: %v\n", err)
		driver.TestStats.StdDev = -1
	}
	driver.TestStats.Mean, err = stats.Mean(driver.TestStats.BenchmarkTimes)
	if err != nil {
		fmt.Printf("Error Calculating Mean: %v\n", err)
		driver.TestStats.Mean = -1
	}
	driver.TestStats.Min, err = stats.Min(driver.TestStats.BenchmarkTimes)
	if err != nil {
		fmt.Printf("Error Calculating Min: %v\n", err)
		driver.TestStats.Min = -1
	}
	driver.TestStats.Pct25, err = stats.Percentile(driver.TestStats.BenchmarkTimes, 25)
	if err != nil {
		fmt.Printf("Error Calculating 25th Pct: %v\n", err)
		driver.TestStats.Pct25 = -1
	}
	driver.TestStats.Pct50, err = stats.Percentile(driver.TestStats.BenchmarkTimes, 50)
	if err != nil {
		fmt.Printf("Error Calculating 50th Pct: %v\n", err)
		driver.TestStats.Pct50 = -1
	}
	driver.TestStats.Pct75, err = stats.Percentile(driver.TestStats.BenchmarkTimes, 75)
	if err != nil {
		fmt.Printf("Error Calculating 75th Pct: %v\n", err)
		driver.TestStats.Pct75 = -1
	}
	driver.TestStats.Pct90, err = stats.Percentile(driver.TestStats.BenchmarkTimes, 90)
	if err != nil {
		fmt.Printf("Error Calculating 90th Pct: %v\n", err)
		driver.TestStats.Pct90 = -1
	}
	driver.TestStats.Max, err = stats.Max(driver.TestStats.BenchmarkTimes)
	if err != nil {
		fmt.Printf("Error Calculating Max: %v\n", err)
		driver.TestStats.Max = -1
	}

	driver.PullStats.StdDev, err = stats.StandardDeviation(driver.PullStats.BenchmarkTimes)
	if err != nil {
		fmt.Printf("PullStats: Error Calculating Std Dev: %v\n", err)
		driver.PullStats.StdDev = -1
	}
	driver.PullStats.Mean, err = stats.Mean(driver.PullStats.BenchmarkTimes)
	if err != nil {
		fmt.Printf("PullStats: Error Calculating Mean: %v\n", err)
		driver.PullStats.Mean = -1
	}
	driver.PullStats.Min, err = stats.Min(driver.PullStats.BenchmarkTimes)
	if err != nil {
		fmt.Printf("PullStats: Error Calculating Min: %v\n", err)
		driver.PullStats.Min = -1
	}
	driver.PullStats.Pct25, err = stats.Percentile(driver.PullStats.BenchmarkTimes, 25)
	if err != nil {
		fmt.Printf("PullStats: Error Calculating 25th Pct: %v\n", err)
		driver.PullStats.Pct25 = -1
	}
	driver.PullStats.Pct50, err = stats.Percentile(driver.PullStats.BenchmarkTimes, 50)
	if err != nil {
		fmt.Printf("PullStats: Error Calculating 50th Pct: %v\n", err)
		driver.PullStats.Pct50 = -1
	}
	driver.PullStats.Pct75, err = stats.Percentile(driver.PullStats.BenchmarkTimes, 75)
	if err != nil {
		fmt.Printf("PullStats: Error Calculating 75th Pct: %v\n", err)
		driver.PullStats.Pct75 = -1
	}
	driver.PullStats.Pct90, err = stats.Percentile(driver.PullStats.BenchmarkTimes, 90)
	if err != nil {
		fmt.Printf("PullStats: Error Calculating 90th Pct: %v\n", err)
		driver.PullStats.Pct90 = -1
	}
	driver.PullStats.Max, err = stats.Max(driver.PullStats.BenchmarkTimes)
	if err != nil {
		fmt.Printf("PullStats: Error Calculating Max: %v\n", err)
		driver.PullStats.Max = -1
	}

	driver.RunTask1Stats.StdDev, err = stats.StandardDeviation(driver.RunTask1Stats.BenchmarkTimes)
	if err != nil {
		fmt.Printf("RunTask1Stats: Error Calculating Std Dev: %v\n", err)
		driver.RunTask1Stats.StdDev = -1
	}
	driver.RunTask1Stats.Mean, err = stats.Mean(driver.RunTask1Stats.BenchmarkTimes)
	if err != nil {
		fmt.Printf("RunTask1Stats: Error Calculating Mean: %v\n", err)
		driver.RunTask1Stats.Mean = -1
	}
	driver.RunTask1Stats.Min, err = stats.Min(driver.RunTask1Stats.BenchmarkTimes)
	if err != nil {
		fmt.Printf("RunTask1Stats: Error Calculating Min: %v\n", err)
		driver.RunTask1Stats.Min = -1
	}
	driver.RunTask1Stats.Pct25, err = stats.Percentile(driver.RunTask1Stats.BenchmarkTimes, 25)
	if err != nil {
		fmt.Printf("RunTask1Stats: Error Calculating 25th Pct: %v\n", err)
		driver.RunTask1Stats.Pct25 = -1
	}
	driver.RunTask1Stats.Pct50, err = stats.Percentile(driver.RunTask1Stats.BenchmarkTimes, 50)
	if err != nil {
		fmt.Printf("RunTask1Stats: Error Calculating 50th Pct: %v\n", err)
		driver.RunTask1Stats.Pct50 = -1
	}
	driver.RunTask1Stats.Pct75, err = stats.Percentile(driver.RunTask1Stats.BenchmarkTimes, 75)
	if err != nil {
		fmt.Printf("RunTask1Stats: Error Calculating 75th Pct: %v\n", err)
		driver.RunTask1Stats.Pct75 = -1
	}
	driver.RunTask1Stats.Pct90, err = stats.Percentile(driver.RunTask1Stats.BenchmarkTimes, 90)
	if err != nil {
		fmt.Printf("RunTask1Stats: Error Calculating 90th Pct: %v\n", err)
		driver.RunTask1Stats.Pct90 = -1
	}
	driver.RunTask1Stats.Max, err = stats.Max(driver.RunTask1Stats.BenchmarkTimes)
	if err != nil {
		fmt.Printf("RunTask1Stats: Error Calculating Max: %v\n", err)
		driver.RunTask1Stats.Max = -1
	}

	driver.RunTask2Stats.StdDev, err = stats.StandardDeviation(driver.RunTask2Stats.BenchmarkTimes)
	if err != nil {
		fmt.Printf("RunTask2Stats: Error Calculating Std Dev: %v\n", err)
		driver.RunTask2Stats.StdDev = -1
	}
	driver.RunTask2Stats.Mean, err = stats.Mean(driver.RunTask2Stats.BenchmarkTimes)
	if err != nil {
		fmt.Printf("RunTask2Stats: Error Calculating Mean: %v\n", err)
		driver.RunTask2Stats.Mean = -1
	}
	driver.RunTask2Stats.Min, err = stats.Min(driver.RunTask2Stats.BenchmarkTimes)
	if err != nil {
		fmt.Printf("RunTask2Stats: Error Calculating Min: %v\n", err)
		driver.RunTask2Stats.Min = -1
	}
	driver.RunTask2Stats.Pct25, err = stats.Percentile(driver.RunTask2Stats.BenchmarkTimes, 25)
	if err != nil {
		fmt.Printf("RunTask2Stats: Error Calculating 25th Pct: %v\n", err)
		driver.RunTask2Stats.Pct25 = -1
	}
	driver.RunTask2Stats.Pct50, err = stats.Percentile(driver.RunTask2Stats.BenchmarkTimes, 50)
	if err != nil {
		fmt.Printf("RunTask2Stats: Error Calculating 50th Pct: %v\n", err)
		driver.RunTask2Stats.Pct50 = -1
	}
	driver.RunTask2Stats.Pct75, err = stats.Percentile(driver.RunTask2Stats.BenchmarkTimes, 75)
	if err != nil {
		fmt.Printf("RunTask2Stats: Error Calculating 75th Pct: %v\n", err)
		driver.RunTask2Stats.Pct75 = -1
	}
	driver.RunTask2Stats.Pct90, err = stats.Percentile(driver.RunTask2Stats.BenchmarkTimes, 90)
	if err != nil {
		fmt.Printf("RunTask2Stats: Error Calculating 90th Pct: %v\n", err)
		driver.RunTask2Stats.Pct90 = -1
	}
	driver.RunTask2Stats.Max, err = stats.Max(driver.RunTask2Stats.BenchmarkTimes)
	if err != nil {
		fmt.Printf("RunTask2Stats: Error Calculating Max: %v\n", err)
		driver.RunTask2Stats.Max = -1
	}
}
