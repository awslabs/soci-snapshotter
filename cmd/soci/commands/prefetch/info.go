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

package prefetch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/awslabs/soci-snapshotter/cmd/soci/commands/internal"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/soci/store"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli/v3"
)

var infoCommand = &cli.Command{
	Name:        "info",
	Usage:       "display a prefetch artifact",
	Description: "get detailed info about a prefetch artifact",
	ArgsUsage:   "<digest>",
	Action: func(ctx context.Context, cmd *cli.Command) error {
		ctx, cancel := internal.AppContext(ctx, cmd)
		defer cancel()

		digest, err := digest.Parse(cmd.Args().First())
		if err != nil {
			return err
		}

		db, err := soci.NewDB(soci.ArtifactsDbPath(cmd.String("root")))
		if err != nil {
			return err
		}

		artifactType, err := db.GetArtifactType(digest.String())
		if err != nil {
			return err
		}
		if artifactType != soci.ArtifactEntryTypePrefetch {
			return fmt.Errorf("the provided digest is not a prefetch artifact (type: %s)", artifactType)
		}

		store, err := store.NewContentStore(internal.ContentStoreOptions(ctx, cmd)...)
		if err != nil {
			return err
		}

		reader, err := store.Fetch(ctx, v1.Descriptor{Digest: digest})
		if err != nil {
			return err
		}
		defer reader.Close()

		artifact, err := soci.UnmarshalPrefetchArtifact(reader)
		if err != nil {
			return err
		}

		// Get artifact entry for additional metadata
		var entry *soci.ArtifactEntry
		db.Walk(func(ae *soci.ArtifactEntry) error {
			if ae.Digest == digest.String() {
				entry = ae
				return fmt.Errorf("found") // stop walking
			}
			return nil
		})

		// Print summary
		fmt.Printf("Digest:        %s\n", digest)
		fmt.Printf("Version:       %s\n", artifact.Version)
		fmt.Printf("Span Ranges:   %d\n", len(artifact.PrefetchSpans))
		if entry != nil {
			fmt.Printf("Layer Digest:  %s\n", entry.OriginalDigest)
			fmt.Printf("Size:          %d bytes\n", entry.Size)
			fmt.Printf("Created:       %s\n", entry.CreatedAt.Format("2006-01-02 15:04:05"))
		}
		fmt.Println()

		// Print detailed span information
		fmt.Println("Prefetch Spans:")
		totalSpans := 0
		for i, span := range artifact.PrefetchSpans {
			spanCount := int(span.EndSpan - span.StartSpan + 1)
			totalSpans += spanCount
			fmt.Printf("  [%d] StartSpan: %d, EndSpan: %d (covers %d spans)\n",
				i, span.StartSpan, span.EndSpan, spanCount)
			if span.Priority != 0 {
				fmt.Printf("      Priority: %d\n", span.Priority)
			}
		}
		fmt.Printf("\nTotal spans to prefetch: %d\n", totalSpans)

		// Optionally print raw JSON
		if cmd.Bool("json") {
			fmt.Println("\nRaw JSON:")
			b, err := json.Marshal(artifact)
			if err != nil {
				return err
			}
			return prettyPrintJSON(b)
		}

		return nil
	},
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "json",
			Usage: "output raw JSON",
		},
	},
}

func prettyPrintJSON(b []byte) error {
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, b, "", "  "); err != nil {
		return err
	}
	_, err := fmt.Println(prettyJSON.String())
	return err
}
