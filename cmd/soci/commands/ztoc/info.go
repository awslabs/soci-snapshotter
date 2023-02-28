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
	"context"
	"encoding/json"
	"fmt"

	"github.com/awslabs/soci-snapshotter/compression"
	"github.com/awslabs/soci-snapshotter/fs/config"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/ztoc"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli"
	"oras.land/oras-go/v2/content/oci"
)

type Info struct {
	Version           string             `json:"version"`
	BuildTool         string             `json:"build_tool"`
	Size              int64              `json:"size"`
	SpanSize          compression.Offset `json:"span_size"`
	NumSpans          compression.SpanID `json:"num_spans"`
	NumFiles          int                `json:"num_files"`
	NumMultiSpanFiles int                `json:"num_multi_span_files"`
	Files             []FileInfo         `json:"files"`
}

type FileInfo struct {
	Filename  string             `json:"filename"`
	Offset    int64              `json:"offset"`
	Size      int64              `json:"size"`
	Type      string             `json:"type"`
	StartSpan compression.SpanID `json:"start_span"`
	EndSpan   compression.SpanID `json:"end_span"`
}

var infoCommand = cli.Command{
	Name:      "info",
	Usage:     "get detailed info about a ztoc",
	ArgsUsage: "<digest>",
	Action: func(cliContext *cli.Context) error {
		digest, err := digest.Parse(cliContext.Args().First())
		if err != nil {
			return err
		}
		db, err := soci.NewDB(soci.ArtifactsDbPath())
		if err != nil {
			return err
		}
		entry, err := db.GetArtifactEntry(digest.String())
		if err != nil {
			return err
		}
		if entry.MediaType == soci.SociIndexArtifactType {
			return fmt.Errorf("the provided digest belongs to a SOCI index. Use `soci index info` to get the detailed information about it")
		}
		storage, err := oci.New(config.SociContentStorePath)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), cliContext.GlobalDuration("timeout"))
		defer cancel()
		reader, err := storage.Fetch(ctx, v1.Descriptor{Digest: digest})
		if err != nil {
			return err
		}
		defer reader.Close()
		ztoc, err := ztoc.Unmarshal(reader)
		if err != nil {
			return err
		}
		gzInfo, err := compression.NewZinfo(compression.Gzip, ztoc.CompressionInfo.Checkpoints)
		if err != nil {
			return err
		}

		multiSpanFiles := 0
		zinfo := Info{
			Version:   ztoc.Version,
			BuildTool: ztoc.BuildToolIdentifier,
			Size:      entry.Size,
			SpanSize:  gzInfo.SpanSize(),
			NumSpans:  ztoc.CompressionInfo.MaxSpanID + 1,
			NumFiles:  len(ztoc.TOC.Metadata),
		}
		for _, v := range ztoc.TOC.Metadata {
			startSpan := gzInfo.UncompressedOffsetToSpanID(v.UncompressedOffset)
			endSpan := gzInfo.UncompressedOffsetToSpanID(v.UncompressedOffset + v.UncompressedSize)
			if startSpan != endSpan {
				multiSpanFiles++
			}
			zinfo.Files = append(zinfo.Files, FileInfo{
				Filename:  v.Name,
				Offset:    int64(v.UncompressedOffset),
				Size:      int64(v.UncompressedSize),
				Type:      v.Type,
				StartSpan: startSpan,
				EndSpan:   endSpan,
			})
		}
		zinfo.NumMultiSpanFiles = multiSpanFiles
		j, err := json.MarshalIndent(zinfo, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(j))
		return nil
	},
}
