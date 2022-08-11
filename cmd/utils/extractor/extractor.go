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

// #include "indexer.h"
// #include <stdlib.h>
// #include <stdio.h>
// #cgo CFLAGS: -I${SRCDIR}/../../../c
// #cgo LDFLAGS: -L${SRCDIR}/../../../out -lindexer -lz
import "C"

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"unsafe"

	"github.com/awslabs/soci-snapshotter/soci"
	"golang.org/x/sync/errgroup"
)

var text = flag.String("text", "", "Name of text file")
var gzFile = flag.String("file", "", "Specify the gzip file")
var ztocFile = flag.String("ztoc", "", "Specify the ztoc file to use")
var remote = flag.String("remote", "", "Specify the remote url for the gzip fle")

func extractFromRemote(url string, ztoc *soci.Ztoc, text string) (string, error) {
	entry, err := soci.GetMetadataEntry(ztoc, text)

	if err != nil {
		return "", err
	}

	if entry.UncompressedSize == 0 {
		return "", nil
	}

	numSpans := entry.SpanEnd - entry.SpanStart + 1
	index := C.blob_to_index(unsafe.Pointer(&ztoc.IndexByteData[0]))
	if index == nil {
		return "", fmt.Errorf("cannot convert blob to gzip_index")
	}
	defer C.free_index(index)
	var bufSize soci.FileSize
	starts := make([]soci.FileSize, numSpans)
	ends := make([]soci.FileSize, numSpans)

	var i soci.SpanId
	for i = 0; i < numSpans; i++ {
		starts[i] = soci.FileSize(C.get_comp_off(index, C.int(i+entry.SpanStart)))

		if i == ztoc.MaxSpanId {
			ends[i] = ztoc.CompressedFileSize
		} else {
			ends[i] = soci.FileSize(C.get_comp_off(index, C.int(i+1+entry.SpanStart)))
		} // This is an ugly hack, we'll revisit this later

		bufSize += (ends[i] - starts[i] + 1)
	}

	start := starts[0]
	// Fetch all span data in parallel
	if entry.FirstSpanHasBits {
		bufSize += 1
		start -= 1
	}

	buf := make([]byte, bufSize)
	eg, ctx := errgroup.WithContext(context.Background())

	for i = 0; i < numSpans; i++ {
		j := i
		eg.Go(func() error {
			rangeStart := starts[j]
			rangeEnd := ends[j]

			if j == 0 && entry.FirstSpanHasBits {
				rangeStart -= 1
			}

			req, err := createRangeRequest(ctx, url, rangeStart, rangeEnd)
			if err != nil {
				return err
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return nil
			}
			if resp.StatusCode != 200 && resp.StatusCode != 206 {
				return fmt.Errorf("got response: %v", resp.StatusCode)
			}
			defer resp.Body.Close()

			// This probably isn't right, but we'll deal with this when it becomes a problem
			b, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("cannot read from response body: %v", err)
			}
			n := copy(buf[rangeStart-start:rangeEnd-start+1], b)
			if n != len(b) {
				return fmt.Errorf("cannot read entire body into buf; read = %d, expected = %d", n, len(b))
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return "", err
	}

	bytes := make([]byte, entry.UncompressedSize)

	ret := C.extract_data_from_buffer(unsafe.Pointer(&buf[0]), C.off_t(len(buf)), index,
		C.off_t(entry.UncompressedOffset), unsafe.Pointer(&bytes[0]),
		C.off_t(entry.UncompressedSize), C.int(entry.SpanStart))
	if ret <= 0 {
		return "", fmt.Errorf("error extracting data; return code: %v", ret)
	}
	return string(bytes), nil
}

func createRangeRequest(ctx context.Context, url string, start, end soci.FileSize) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	header := make(http.Header)
	bytesToFetch := fmt.Sprintf("bytes=%d-%d", start, end)
	header.Add("Range", bytesToFetch)
	req.Header = header
	return req, nil
}

func main() {
	flag.Parse()

	if *text == "" || *ztocFile == "" {
		fmt.Println("text/ztoc file not specified")
		os.Exit(-1)
	}

	if *gzFile == "" && *remote == "" {
		fmt.Println("either --file or --remote must be passed")
		os.Exit(-1)
	}

	ztoc, err := soci.GetZtocFromFile(*ztocFile)
	if err != nil {
		fmt.Println("unable to deserialize ztoc")
		os.Exit(-1)
	}

	var str string
	if *gzFile != "" {
		str, err = soci.ExtractFromTarGz(*gzFile, ztoc, *text)
		if err != nil {
			fmt.Printf("unable to extract from tar gz: %v\n", err)
			os.Exit(-1)
		}
	} else {
		str, err = extractFromRemote(*remote, ztoc, *text)
		if err != nil {
			fmt.Printf("unable to extract from remote: %v\n", err)
			os.Exit(-1)
		}
	}

	fmt.Printf("%s", str)
}
