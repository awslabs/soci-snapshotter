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

package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"slices"
	"sort"
	"strings"

	"github.com/awslabs/soci-snapshotter/cmd/soci/commands/internal"
	"github.com/awslabs/soci-snapshotter/fs"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/platforms"
	dockercliconfig "github.com/docker/cli/cli/config"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli/v3"
	oraslib "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

const (
	maxConcurrentUploadsFlag = "max-concurrent-uploads"

	// defaultMaxConcurrentUploads is the default number of copy tasks used to upload the SOCI artifacts.
	// This aligns with the ORAS Go library default value.
	// Reference: https://github.com/oras-project/oras-go/blob/850a24737c15927603d1cecddc74c87f5f3377f4/copy.go#L38
	defaultMaxConcurrentUploads = 3
)

type pushDescs struct {
	sociIndexDesc  soci.IndexDescriptorInfo
	imageIndexDesc *ocispec.Descriptor
}

// PushCommand is a command to push an image artifacts from local content store to the remote repository
var PushCommand = &cli.Command{
	Name:      "push",
	Usage:     "push SOCI artifacts to a registry",
	ArgsUsage: "[flags] <ref>",
	Description: `Push SOCI artifacts to a registry by image reference.
If multiple soci indices exist for the given image, the most recent one will be pushed.

After pushing the soci artifacts, they should be available in the registry. Soci artifacts will be pushed only
if they are available in the snapshotter's local content store.
`,
	Flags: append(append(append(
		internal.RegistryFlags,
		internal.SnapshotterFlags...),
		internal.PlatformFlags...),
		internal.ExistingIndexFlag,
		&cli.Uint64Flag{
			Name:  maxConcurrentUploadsFlag,
			Usage: fmt.Sprintf("Max concurrent uploads. Default is %d", defaultMaxConcurrentUploads),
			Value: defaultMaxConcurrentUploads,
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "quiet mode",
		},
	),
	Action: func(ctx context.Context, cmd *cli.Command) error {
		ref := cmd.Args().First()
		if ref == "" {
			return errors.New("please provide an image reference to push")
		}

		quiet := cmd.Bool("quiet")
		client, ctx, cancel, err := internal.NewClient(ctx, cmd)
		if err != nil {
			return err
		}
		defer cancel()

		cs := client.ContentStore()
		is := client.ImageService()
		img, err := is.Get(ctx, ref)
		if err != nil {
			return err
		}

		ps, err := internal.GetPlatforms(ctx, cmd, img, cs)
		if err != nil {
			return err
		}
		if len(ps) == 0 {
			ps = append(ps, platforms.DefaultSpec())
		}

		artifactsDb, err := soci.NewDB(soci.ArtifactsDbPath(cmd.String("root")))
		if err != nil {
			return err
		}

		refspec, err := reference.Parse(ref)
		if err != nil {
			return err
		}

		dst, err := remote.NewRepository(refspec.Locator)
		if err != nil {
			return err
		}
		authClient := auth.DefaultClient

		var username string
		var secret string
		if cmd.IsSet("username") {
			username = cmd.String("username")
			if i := strings.IndexByte(username, ':'); i > 0 {
				secret = username[i+1:]
				username = username[0:i]
			}
		} else {
			cf := dockercliconfig.LoadDefaultConfigFile(io.Discard)
			if cf.ContainsAuth() {
				if ac, err := cf.GetAuthConfig(refspec.Hostname()); err == nil {
					username = ac.Username
					secret = ac.Password
				}
			}
		}

		authClient.Credential = func(_ context.Context, host string) (auth.Credential, error) {
			return auth.Credential{
				Username: username,
				Password: secret,
			}, nil
		}

		src, err := store.NewContentStore(internal.ContentStoreOptions(ctx, cmd)...)
		if err != nil {
			return fmt.Errorf("cannot create local content store: %w", err)
		}

		dst.Client = authClient

		dst.PlainHTTP = cmd.Bool("plain-http")

		if cmd.Bool("debug") {
			dst.Client = &debugClient{client: authClient}
		} else {
			dst.Client = authClient
		}

		existingIndexOption := cmd.String(internal.ExistingIndexFlagName)
		if !internal.SupportedArg(existingIndexOption, internal.SupportedExistingIndexOptions) {
			return fmt.Errorf("unexpected value for flag %s: %s, expected types %v",
				internal.ExistingIndexFlagName, existingIndexOption, internal.SupportedExistingIndexOptions)
		}

		options := oraslib.DefaultCopyGraphOptions
		if value := cmd.Uint64(maxConcurrentUploadsFlag); value == 0 {
			options.Concurrency = defaultMaxConcurrentUploads
		} else if value > math.MaxInt {
			if !quiet {
				fmt.Printf("warning: overflow for setting --%s=%d; defaulting to %d", maxConcurrentUploadsFlag, value, defaultMaxConcurrentUploads)
			}
			options.Concurrency = defaultMaxConcurrentUploads
		} else {
			options.Concurrency = int(value)
		}
		options.PreCopy = func(_ context.Context, desc ocispec.Descriptor) error {
			if !quiet {
				fmt.Printf("pushing artifact with digest: %v\n", desc.Digest)
			}
			return nil
		}
		options.PostCopy = func(_ context.Context, desc ocispec.Descriptor) error {
			if !quiet {
				fmt.Printf("successfully pushed artifact with digest: %v\n", desc.Digest)
			}
			return nil
		}
		options.OnCopySkipped = func(ctx context.Context, desc ocispec.Descriptor) error {
			if !quiet {
				fmt.Printf("skipped artifact with digest: %v\n", desc.Digest)
			}
			return nil
		}

		var toPush []pushDescs
		var foundV2Indexes bool
		for _, platform := range ps {
			indexDescriptors, imgManifestDesc, err := soci.GetIndexDescriptorCollection(ctx, cs, artifactsDb, img, []ocispec.Platform{platform})
			if err != nil {
				return err
			}
			v1IndexDescriptors := slices.DeleteFunc(indexDescriptors, func(desc soci.IndexDescriptorInfo) bool {
				if desc.ArtifactType == soci.SociIndexArtifactTypeV2 {
					foundV2Indexes = true
					return true
				}
				return false
			})

			if foundV2Indexes {
				fmt.Println("[WARN] Skippping SOCI index manifest v2. You should push the whole image with higher level tools like finch or nerdctl.")
			}
			if len(v1IndexDescriptors) == 0 {
				return fmt.Errorf("could not find any soci index manifest v1 to push for platform %v", platforms.Format(platform))
			}

			sort.Slice(v1IndexDescriptors, func(i, j int) bool {
				return v1IndexDescriptors[i].CreatedAt.Before(v1IndexDescriptors[j].CreatedAt)
			})
			indexDesc := v1IndexDescriptors[len(v1IndexDescriptors)-1]
			toPush = append(toPush, pushDescs{
				sociIndexDesc:  indexDesc,
				imageIndexDesc: imgManifestDesc,
			})
		}

		for i := range len(toPush) {
			indexDesc := toPush[i].sociIndexDesc
			imgManifestDesc := toPush[i].imageIndexDesc
			if existingIndexOption != internal.Allow {
				if !quiet {
					fmt.Println("checking if a soci index already exists in remote repository...")
				}
				client := fs.NewOCIArtifactClient(dst)
				referrers, err := client.AllReferrers(ctx, ocispec.Descriptor{Digest: imgManifestDesc.Digest})
				if err != nil && !errors.Is(err, fs.ErrNoReferrers) {
					return fmt.Errorf("failed to fetch list of referrers: %w", err)
				}
				if len(referrers) > 0 {
					var foundMessage string
					if len(referrers) > 1 {
						foundMessage = "multiple soci indices found in remote repository"
					} else {
						foundMessage = fmt.Sprintf("soci index found in remote repository with digest: %s", referrers[0].Digest.String())
					}
					switch existingIndexOption {
					case internal.Skip:
						if !quiet {
							fmt.Printf("%s: skipping pushing artifacts for image manifest: %s\n", foundMessage, imgManifestDesc.Digest.String())
						}
						continue
					case internal.Warn:
						fmt.Printf("[WARN] %s: pushing index anyway\n", foundMessage)
						// Fall through and attempt to push the index anyway
					}
				}

			}

			if quiet {
				fmt.Println(indexDesc.Digest.String())
			} else {
				fmt.Printf("pushing soci index with digest: %v\n", indexDesc.Digest)
			}

			err = oraslib.CopyGraph(ctx, src, dst, indexDesc.Descriptor, options)
			if err != nil {
				return fmt.Errorf("error pushing graph to remote: %w", err)
			}

		}
		return nil
	},
}

type debugClient struct {
	client remote.Client
}

func (c *debugClient) Do(req *http.Request) (*http.Response, error) {
	fmt.Printf("http req %s %s\n", req.Method, req.URL)
	res, err := c.client.Do(req)
	if err != nil {
		fmt.Printf("http err %v\n", err)
	} else {
		fmt.Printf("http res %s\n", res.Status)
	}
	return res, err
}
