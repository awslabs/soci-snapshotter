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

/*
   Copyright The containerd Authors.

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

package testutil

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/opencontainers/go-digest"
)

// ApplyTextTemplate applies the config to the specified template.
func ApplyTextTemplate(temp string, config interface{}) (string, error) {
	data, err := ApplyTextTemplateErr(temp, config)
	if err != nil {
		return "", fmt.Errorf("failed to apply config %v to template", config)
	}
	return string(data), nil
}

// ApplyTextTemplateErr applies the config to the specified template.
func ApplyTextTemplateErr(temp string, conf interface{}) ([]byte, error) {
	var buf bytes.Buffer
	if err := template.Must(template.New(digest.FromString(temp).String()).Parse(temp)).Execute(&buf, conf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
