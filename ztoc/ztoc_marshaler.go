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

package ztoc

import (
	"bytes"
	"io"

	ztoc_flatbuffers "github.com/awslabs/soci-snapshotter/ztoc/fbs/ztoc"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Marshal serializes Ztoc to its flatbuffers schema and returns a reader along with the descriptor (digest and size only).
// If not successful, it will return an error.
func Marshal(ztoc *Ztoc) (io.Reader, ocispec.Descriptor, error) {
	var flatbuf []byte
	var err error
	if ztoc.Version == "0.9" {
		flatbuf, err = ztocToFlatbuffer09(ztoc)
	} else {
		flatbuf, err = ztocToFlatbuffer(ztoc)
	}
	if err != nil {
		return nil, ocispec.Descriptor{}, err
	}

	buf := bytes.NewReader(flatbuf)
	dgst := digest.FromBytes(flatbuf)
	size := len(flatbuf)
	return buf, ocispec.Descriptor{
		Digest: dgst,
		Size:   int64(size),
	}, nil
}

// Unmarshal takes the reader with flatbuffers byte stream and deserializes it ztoc.
// In case if there's any error situation during deserialization from flatbuffers, there will be an error returned.
func Unmarshal(serializedZtoc io.Reader) (*Ztoc, error) {
	flatbuf, err := io.ReadAll(serializedZtoc)
	if err != nil {
		return nil, err
	}

	ztocFlatbuf := ztoc_flatbuffers.GetRootAsZtoc(flatbuf, 0)
	version := string(ztocFlatbuf.Version())

	if version == "0.9" {
		return flatbufToZtoc09(flatbuf)
	}

	return flatbufToZtoc(flatbuf)
}
