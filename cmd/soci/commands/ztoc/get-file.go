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

	"github.com/awslabs/soci-snapshotter/cmd/soci/commands/internal"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/awslabs/soci-snapshotter/ztoc"
	"github.com/containerd/containerd/content"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli/v2"
)

var getFileCommand = &cli.Command{
	Name:      "get-file",
	Usage:     "retrieve a file from a local image layer using a specified ztoc",
	ArgsUsage: "<digest> <file>",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "the file to write the extracted content. Defaults to stdout",
		},
	},
	Action: func(cliContext *cli.Context) error {
		args := cliContext.Args().Slice()
		if len(args) != 2 {
			return errors.New("please provide both a ztoc digest and a filename to extract")
		}

		ztocDigest, err := digest.Parse(args[0])
		if err != nil {
			return err
		}
		file := args[1]

		client, ctx, cancel, err := internal.NewClient(cliContext)
		if err != nil {
			return err
		}
		defer cancel()

		toc, err := getZtoc(ctx, cliContext, ztocDigest)
		if err != nil {
			return err
		}

		artifactsDB, err := soci.NewDB(soci.ArtifactsDbPath(cliContext.String("root")))
		if err != nil {
			return err
		}

		layerReader, err := getLayer(ctx, artifactsDB, ztocDigest, client.ContentStore())
		if err != nil {
			return err
		}
		defer layerReader.Close()

		data, err := toc.ExtractFile(io.NewSectionReader(layerReader, 0, int64(toc.CompressedArchiveSize)), file)
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

func getZtoc(ctx context.Context, cliContext *cli.Context, d digest.Digest) (*ztoc.Ztoc, error) {
	blobStore, err := store.NewContentStore(internal.ContentStoreOptions(cliContext)...)
	if err != nil {
		return nil, err
	}

	reader, err := blobStore.Fetch(ctx, v1.Descriptor{Digest: d})
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return ztoc.Unmarshal(reader)
}

func getLayer(ctx context.Context, metadata *soci.ArtifactsDb, ztocDigest digest.Digest, cs content.Store) (content.ReaderAt, error) {
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
