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
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/awslabs/soci-snapshotter/fs/config"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/containerd/containerd/content"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli"
	"oras.land/oras-go/v2/content/oci"
)

var getFileCommand = cli.Command{
	Name:      "get-file",
	Usage:     "retrieve a file from a local image layer using a specified ztoc",
	ArgsUsage: "<digest> <file>",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "output, o",
			Usage: "the file to write the extracted content. Defaults to stdout",
		},
	},
	Action: func(cliContext *cli.Context) error {
		if len(cliContext.Args()) != 2 {
			return errors.New("please provide both a ztoc digest and a filename to extract")
		}

		ztocDigest, err := digest.Parse(cliContext.Args()[0])
		if err != nil {
			return err
		}
		file := cliContext.Args()[1]

		client, ctx, cancel, err := commands.NewClient(cliContext)
		if err != nil {
			return err
		}
		defer cancel()

		ztoc, err := getZtoc(ctx, ztocDigest)
		if err != nil {
			return err
		}

		layerReader, err := getLayer(ctx, ztocDigest, client.ContentStore())
		if err != nil {
			return err
		}
		defer layerReader.Close()

		fileMetadata, err := soci.GetMetadataEntry(ztoc, file)
		if err != nil {
			return err
		}
		extractConfig := soci.FileExtractConfig{
			UncompressedSize:      fileMetadata.UncompressedSize,
			UncompressedOffset:    fileMetadata.UncompressedOffset,
			Checkpoints:           ztoc.CompressionInfo.Checkpoints,
			CompressedArchiveSize: ztoc.CompressedArchiveSize,
			MaxSpanID:             ztoc.CompressionInfo.MaxSpanID,
		}

		data, err := soci.ExtractFile(io.NewSectionReader(layerReader, 0, int64(ztoc.CompressedArchiveSize)), &extractConfig)
		if err != nil {
			return err
		}

		outfile := cliContext.String("output")
		if outfile != "" {
			os.WriteFile(outfile, data, 0)
			return nil
		}
		fmt.Println(string(data))
		return nil
	},
}

func getZtoc(ctx context.Context, d digest.Digest) (*soci.Ztoc, error) {
	blobStore, err := oci.New(config.SociContentStorePath)
	if err != nil {
		return nil, err
	}

	reader, err := blobStore.Fetch(ctx, v1.Descriptor{Digest: d})
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return soci.GetZtoc(reader)
}

func getLayer(ctx context.Context, ztocDigest digest.Digest, cs content.Store) (content.ReaderAt, error) {
	metadata, err := soci.NewDB()
	if err != nil {
		return nil, err
	}
	artifact, err := metadata.GetArtifactEntry(ztocDigest.String())
	if err != nil {
		return nil, err
	}
	layerDigest, err := digest.Parse(artifact.OriginalDigest)
	if err != nil {
		return nil, err
	}

	return cs.ReaderAt(ctx, v1.Descriptor{Digest: layerDigest})
}
