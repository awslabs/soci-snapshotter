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

package internal

import (
	"context"
	"fmt"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli/v2"
)

const (
	PlatformFlagKey     = "platform"
	AllPlatformsFlagKey = "all-platforms"
)

var PlatformFlags = []cli.Flag{
	&cli.BoolFlag{
		Name:  AllPlatformsFlagKey,
		Usage: "",
	},
	&cli.StringSliceFlag{
		Name:    PlatformFlagKey,
		Aliases: []string{"p"},
		Usage:   "",
	},
}

// GetPlatforms returns the set of platforms from a cli.Context
// The order of preference is:
// 1) all platforms supported by the image if the `all-plaforms` flag is set
// 2) the set of platforms specified by the `platform` flag
// 3) An empty platform slice. The consumer is responsible for setting an appropriate platform
//
// This method is not suitable for situations where the default should be all supported platforms (e.g. the `soci index list` command)
func GetPlatforms(ctx context.Context, cliContext *cli.Context, img images.Image, cs content.Store) ([]ocispec.Platform, error) {
	if cliContext.Bool(AllPlatformsFlagKey) {
		return images.Platforms(ctx, cs, img.Target)
	}
	ps := cliContext.StringSlice(PlatformFlagKey)
	if len(ps) == 0 {
		return []ocispec.Platform{}, nil
	}
	var result []ocispec.Platform
	for _, p := range ps {
		platform, err := platforms.Parse(p)
		if err != nil {
			return nil, fmt.Errorf("could not parse platform %s: %w", p, err)
		}
		result = append(result, platform)
	}
	return result, nil
}
