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
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/awslabs/soci-snapshotter/benchmark/framework"
)

var (
	benchmarkOutput = "output.json"
)

func main() {
	commit := os.Args[1]
	var drivers []framework.BenchmarkTestDriver
	drivers = append(drivers, framework.BenchmarkTestDriver{
		TestName:      "TwoSecondSleep",
		NumberOfTests: 10,
		TestFunction: func(b *testing.B) {
			BenchmarkExample(b, false, 2)
		},
	})
	drivers = append(drivers, framework.BenchmarkTestDriver{
		TestName:      "TwoSecondRandSleep",
		NumberOfTests: 10,
		TestFunction: func(b *testing.B) {
			BenchmarkExample(b, true, 2)
		},
	})
	drivers = append(drivers, framework.BenchmarkTestDriver{
		TestName:      "ThreeSecondSleep",
		NumberOfTests: 10,
		TestFunction: func(b *testing.B) {
			BenchmarkExample(b, false, 3)
		},
	})

	benchmarks := framework.BenchmarkFramework{
		OutputFile: benchmarkOutput,
		CommitID:   commit,
		Drivers:    drivers,
	}
	benchmarks.Run()
}

func BenchmarkExample(b *testing.B, isRand bool, baseSeconds int64) {
	sleepTime := time.Duration(baseSeconds)
	if isRand {
		rand.Seed(time.Now().UnixNano())
		sleepTime = time.Duration(rand.Int63n(baseSeconds))
	}
	time.Sleep(sleepTime * time.Second)
}
