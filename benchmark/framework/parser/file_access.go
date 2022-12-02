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

package bparser

import (
	"encoding/json"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

var (
	outputDir           = "./output/"
	FileAccessDir       = outputDir + "file_access_logs/"
	sociLogs            = outputDir + "soci-snapshotter-stderr"
	containerdLogs      = outputDir + "containerd-stderr"
	sociLogsFuseMessage = "FUSE operation"
)

type FileAccessPatterns struct {
	ImageName           string         `json:"ImageName"`
	ContainerStartTime  time.Time      `json:"containerStartTime"`
	TotalOperationCount map[string]int `json:"TotalOperationCounts"`
	Operations          []Operation    `json:"operations"`
}

type SociLog struct {
	Msg string `json:"msg"`
}
type BaseOperation struct {
	Level     string    `json:"level"`
	Msg       string    `json:"msg"`
	Operation string    `json:"operation"`
	Path      string    `json:"path"`
	Time      time.Time `json:"time"`
}

type Operation struct {
	Operation                 string `json:"operation"`
	Path                      string `json:"path"`
	FirstAccessTimeAfterStart string `json:"firstAccessTimeAfterStart"`
	Count                     int    `json:"count"`
}

func ParseFileAccesses(imageName string) error {
	startTime, err := getTaskStartTime()
	if err != nil {
		return err
	}
	totalCounts := make(map[string]int)
	fa := FileAccessPatterns{
		ImageName:           imageName,
		ContainerStartTime:  *startTime,
		TotalOperationCount: totalCounts,
	}
	file, err := os.Open(sociLogs)
	if err != nil {
		return err
	}

	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	logs := strings.Split(string(b), "\n")

	m := make(map[string]Operation)
	for _, log := range logs {
		sociLog := SociLog{}
		if log != "" {
			err := json.Unmarshal([]byte(log), &sociLog)
			if err != nil {
				return err
			}

		}
		if sociLog.Msg == sociLogsFuseMessage {
			var tempOperation BaseOperation
			err := json.Unmarshal([]byte(log), &tempOperation)
			if err != nil {
				return err
			}
			op := tempOperation.Operation + tempOperation.Path

			val, ok := m[op]

			if ok {
				val.Count = val.Count + 1
				m[op] = val

			} else {
				m[op] = Operation{
					Operation:                 tempOperation.Operation,
					Path:                      tempOperation.Path,
					FirstAccessTimeAfterStart: tempOperation.Time.Sub(fa.ContainerStartTime).String(),
					Count:                     1,
				}
			}
			count, ok := fa.TotalOperationCount[tempOperation.Operation]
			if ok {
				fa.TotalOperationCount[tempOperation.Operation] = count + 1
			} else {
				fa.TotalOperationCount[tempOperation.Operation] = 1
			}
		}
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		t1, _ := time.ParseDuration(m[keys[i]].FirstAccessTimeAfterStart)
		t2, _ := time.ParseDuration(m[keys[j]].FirstAccessTimeAfterStart)
		return t1 < t2
	})

	for _, key := range keys {
		fa.Operations = append(fa.Operations, m[key])
	}
	json, err := json.MarshalIndent(fa, "", " ")
	if err != nil {
		return err
	}
	imageFileAccessLogPath := FileAccessDir + imageName + "_access_patterns"
	err = os.WriteFile(imageFileAccessLogPath, json, 0644)
	if err != nil {
		return err
	}

	return nil
}

func getTaskStartTime() (*time.Time, error) {
	var taskStartTime time.Time
	file, err := os.Open(containerdLogs)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	logs := strings.Split(string(data), "\n")
	for i := len(logs) - 1; i >= 0; i-- {
		log := logs[i]
		if strings.Contains(log, "/tasks/start") {
			l := strings.Split(log, " ")
			temp := strings.ReplaceAll(l[0], "time=", "")
			taskStartTime, err = time.Parse(time.RFC3339, temp[1:len(temp)-1])
			if err != nil {
				return nil, err
			}
		}
	}
	return &taskStartTime, nil
}
