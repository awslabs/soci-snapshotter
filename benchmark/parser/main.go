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
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type BenchmarkEvent struct {
	Benchmark string `json:"benchmark"`
	Event     string `json:"event"`
	TestName  string `json:"test_name"`
	UUID      string `json:"uuid"`
	Timestamp string `json:"time"`
}

type ProfileEvent struct {
	EventName string
	StartTime time.Time
	StopTime  time.Time
}

type BenchmarkProfile struct {
	UUID     string
	TestName string
	Events   map[string]ProfileEvent
}

func main() {
	logMap := make(map[string]BenchmarkProfile)
	logFileName := os.Args[1]
	logFile, err := os.Open(logFileName)
	if err != nil {
		panic("Cannot open " + logFileName)
	}
	defer logFile.Close()
	fileScanner := bufio.NewScanner(logFile)

	fileScanner.Split(bufio.ScanLines)

	for fileScanner.Scan() {
		line := fileScanner.Text()
		if strings.Contains(line, "benchmark") {
			parseLogLineToMap(fileScanner.Text(), logMap)
		}
	}
	if err := fileScanner.Err(); err != nil {
		panic("scan error: " + err.Error())
	}

	printLogMap(logMap)
}

func parseLogLineToMap(logline string, logmap map[string]BenchmarkProfile) {
	var event BenchmarkEvent
	json.Unmarshal([]byte(logline), &event)

	testData, ok := logmap[event.UUID]
	if !ok {
		testData = BenchmarkProfile{
			UUID:     event.UUID,
			TestName: event.TestName,
			Events:   make(map[string]ProfileEvent),
		}
		logmap[event.UUID] = testData
	}
	profileEvent, ok := testData.Events[event.Benchmark]
	if !ok {
		profileEvent = ProfileEvent{
			EventName: event.Benchmark,
		}
		testData.Events[event.Benchmark] = profileEvent
	}
	if event.Event == "Start" {
		startTime, _ := time.Parse(time.RFC3339, event.Timestamp)
		profileEvent.StartTime = startTime
	}
	if event.Event == "Stop" {
		stopTime, _ := time.Parse(time.RFC3339, event.Timestamp)
		profileEvent.StopTime = stopTime
	}
	testData.Events[event.Benchmark] = profileEvent
	fmt.Printf("profile Event: %v\n", profileEvent)
}

func printLogMap(logMap map[string]BenchmarkProfile) {
	for uuid, profile := range logMap {
		fmt.Printf("Test: %s ID: %s Key: %s\n", profile.TestName, profile.UUID, uuid)
		for benchmarkName, benchmark := range profile.Events {
			fmt.Printf("    Event: %v Key: %s\n", benchmark, benchmarkName)
		}
	}
}
