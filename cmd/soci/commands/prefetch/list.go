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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/awslabs/soci-snapshotter/cmd/soci/commands/internal"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/soci/store"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli/v3"
)

var listCommand = &cli.Command{
	Name:        "list",
	Aliases:     []string{"ls"},
	Usage:       "list prefetch artifacts",
	Description: "list all prefetch artifacts in the local store",
	Action: func(ctx context.Context, cmd *cli.Command) error {
		ctx, cancel := internal.AppContext(ctx, cmd)
		defer cancel()

		db, err := soci.NewDB(soci.ArtifactsDbPath(cmd.String("root")))
		if err != nil {
			return err
		}

		contentStore, err := store.NewContentStore(internal.ContentStoreOptions(ctx, cmd)...)
		if err != nil {
			return err
		}

		type prefetchInfo struct {
			Digest      string
			LayerDigest string
			TotalSpans  int
			CreatedAt   time.Time
		}

		var prefetches []prefetchInfo

		err = db.Walk(func(entry *soci.ArtifactEntry) error {
			if entry.Type != soci.ArtifactEntryTypePrefetch {
				return nil
			}

			// Parse digest string to digest.Digest
			dgst, err := digest.Parse(entry.Digest)
			if err != nil {
				// If digest is invalid, still list it but with limited info
				prefetches = append(prefetches, prefetchInfo{
					Digest:      entry.Digest,
					LayerDigest: entry.OriginalDigest,
					TotalSpans:  -1,
					CreatedAt:   entry.CreatedAt,
				})
				return nil
			}

			// Fetch and parse the prefetch artifact to get span count
			reader, err := contentStore.Fetch(ctx, v1.Descriptor{
				Digest: dgst,
			})
			if err != nil {
				// If we can't fetch it, still list it but with limited info
				prefetches = append(prefetches, prefetchInfo{
					Digest:      entry.Digest,
					LayerDigest: entry.OriginalDigest,
					TotalSpans:  -1,
					CreatedAt:   entry.CreatedAt,
				})
				return nil
			}
			defer reader.Close()

			artifact, err := soci.UnmarshalPrefetchArtifact(reader)
			if err != nil {
				prefetches = append(prefetches, prefetchInfo{
					Digest:      entry.Digest,
					LayerDigest: entry.OriginalDigest,
					TotalSpans:  -1,
					CreatedAt:   entry.CreatedAt,
				})
				return nil
			}

			totalSpans := 0
			for _, span := range artifact.PrefetchSpans {
				totalSpans += int(span.EndSpan - span.StartSpan + 1)
			}

			prefetches = append(prefetches, prefetchInfo{
				Digest:      entry.Digest,
				LayerDigest: entry.OriginalDigest,
				TotalSpans:  totalSpans,
				CreatedAt:   entry.CreatedAt,
			})

			return nil
		})

		if err != nil {
			return err
		}

		if cmd.Bool("json") {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(prefetches)
		}

		// Print as table
		w := tabwriter.NewWriter(os.Stdout, 8, 8, 4, ' ', 0)
		fmt.Fprintln(w, "DIGEST\tLAYER DIGEST\tSPANS\tCREATED\t")

		for _, p := range prefetches {
			spansStr := fmt.Sprintf("%d", p.TotalSpans)
			if p.TotalSpans == -1 {
				spansStr = "N/A"
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t\n",
				p.Digest,
				p.LayerDigest,
				spansStr,
				getDuration(p.CreatedAt),
			)
		}

		return w.Flush()
	},
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "json",
			Usage: "output in JSON format",
		},
	},
}

func getDuration(t time.Time) string {
	if t.IsZero() {
		return "n/a"
	}
	return fmt.Sprintf("%s ago", time.Since(t).Round(time.Second).String())
}
