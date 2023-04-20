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
	"fmt"
	"io"
	"os"

	"github.com/awslabs/soci-snapshotter/ztoc/compression"
	"github.com/opencontainers/go-digest"
)

// ZinfoBuilder builds the `zinfo` part of a ztoc. This interface should be
// implemented for each compression algorithm we support.
type ZinfoBuilder interface {
	// ZinfoFromFile builds zinfo given a compressed tar filename and span size, and calculate the size of the file.
	ZinfoFromFile(filename string, spanSize int64) (zinfo CompressionInfo, fs compression.Offset, err error)
}

type gzipZinfoBuilder struct{}

// ZinfoFromFile creates zinfo for a gzip file. The underlying zinfo object (i.e. `GzipZinfo`)
// is stored in `CompressionInfo.Checkpoints` as byte slice.
func (gzb gzipZinfoBuilder) ZinfoFromFile(filename string, spanSize int64) (zinfo CompressionInfo, fs compression.Offset, err error) {
	index, err := compression.NewZinfoFromFile(compression.Gzip, filename, spanSize)
	if err != nil {
		return
	}
	defer index.Close()

	fs, err = getFileSize(filename)
	if err != nil {
		return
	}

	digests, err := getPerSpanDigests(filename, int64(fs), index)
	if err != nil {
		return
	}

	checkpoints, err := index.Bytes()
	if err != nil {
		return
	}

	return CompressionInfo{
		MaxSpanID:            index.MaxSpanID(),
		SpanDigests:          digests,
		Checkpoints:          checkpoints,
		CompressionAlgorithm: compression.Gzip,
	}, fs, nil
}

type tarZinfoBuilder struct{}

func (tzb tarZinfoBuilder) ZinfoFromFile(filename string, spanSize int64) (zinfo CompressionInfo, fs compression.Offset, err error) {
	index, err := compression.NewZinfoFromFile(compression.Uncompressed, filename, spanSize)
	if err != nil {
		return
	}
	defer index.Close()

	fs, err = getFileSize(filename)
	if err != nil {
		return
	}

	digests, err := getPerSpanDigests(filename, int64(fs), index)
	if err != nil {
		return
	}

	checkpoints, err := index.Bytes()
	if err != nil {
		return
	}

	return CompressionInfo{
		MaxSpanID:            index.MaxSpanID(),
		SpanDigests:          digests,
		Checkpoints:          checkpoints,
		CompressionAlgorithm: compression.Uncompressed,
	}, fs, nil
}

func getPerSpanDigests(filename string, fileSize int64, index compression.Zinfo) ([]digest.Digest, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("could not open file for reading: %w", err)
	}
	defer file.Close()

	var digests []digest.Digest
	var i compression.SpanID
	maxSpanID := index.MaxSpanID()
	for i = 0; i <= maxSpanID; i++ {
		startOffset := index.StartCompressedOffset(i)
		endOffset := index.EndCompressedOffset(i, compression.Offset(fileSize))

		section := io.NewSectionReader(file, int64(startOffset), int64(endOffset-startOffset))
		dgst, err := digest.FromReader(section)
		if err != nil {
			return nil, fmt.Errorf("unable to compute digest for section; start=%d, end=%d, file=%s, size=%d", startOffset, endOffset, filename, fileSize)
		}
		digests = append(digests, dgst)
	}
	return digests, nil
}

func getFileSize(file string) (compression.Offset, error) {
	st, err := os.Stat(file)
	if err != nil {
		return 0, err
	}
	return compression.Offset(st.Size()), nil
}
