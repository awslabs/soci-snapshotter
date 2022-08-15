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

package image

import (
	"fmt"

	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/urfave/cli"
)

var listIndicesCommand = cli.Command{
	Name:      "list-indices",
	Usage:     "Get a list of SOCI indices from an image ref.",
	ArgsUsage: "[flags] <ref>",
	Action: func(cliContext *cli.Context) error {
		ref := cliContext.Args().First()

		if ref == "" {
			return fmt.Errorf("please provide an image reference")
		}

		client, ctx, cancel, err := commands.NewClient(cliContext)
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

		indexDescriptors, err := soci.GetIndexDescriptorCollection(ctx, cs, img)
		if err != nil {
			return err
		}

		if len(indexDescriptors) == 0 {
			return fmt.Errorf("could not find any soci index digests for the provided ref")
		}

		fmt.Printf("%v", indexDescriptors[len(indexDescriptors)-1].Digest)
		return nil
	},
}
