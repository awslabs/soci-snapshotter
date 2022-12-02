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

package benchmark

import (
	"encoding/csv"
	"errors"
	"os"
)

type ImageDescriptor struct {
	ShortName            string
	ImageRef             string
	SociIndexManifestRef string
	ReadyLine            string
}

func GetImageListFromCsv(csvLoc string) ([]ImageDescriptor, error) {
	csvFile, err := os.Open(csvLoc)
	if err != nil {
		return nil, err
	}
	csv, err := csv.NewReader(csvFile).ReadAll()
	if err != nil {
		return nil, err
	}
	var images []ImageDescriptor
	for _, image := range csv {
		if len(image) < 4 {
			return nil, errors.New("image input is not sufficient")
		}
		images = append(images, ImageDescriptor{
			ShortName:            image[0],
			ImageRef:             image[1],
			SociIndexManifestRef: image[2],
			ReadyLine:            image[3],
		})
	}
	return images, nil
}
