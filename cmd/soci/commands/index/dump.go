package index

import (
	"fmt"
	"strconv"

	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/urfave/cli"
)

var dumpCommand = cli.Command{
	Name:        "dump",
	Aliases:     []string{"dump"},
	Usage:       "dump the artifact db",
	Description: "prints the contents of the artifact db to stdout",
	Flags:       []cli.Flag{},
	Action: func(cliContext *cli.Context) error {
		db, err := soci.NewDB(soci.ArtifactsDbPath())
		if err != nil {
			return err
		}
		db.Walk(func(ae *soci.ArtifactEntry) error {
			fmt.Println(" ArtifactEntry:")
			fmt.Println("          Size: " + strconv.FormatInt(ae.Size, 10))
			fmt.Println("        Digest: " + ae.Digest)
			fmt.Println("OriginalDigest: " + ae.OriginalDigest)
			fmt.Println("   ImageDigest: " + ae.ImageDigest)
			fmt.Println("      Platform: " + ae.Platform)
			fmt.Println("      Location: " + ae.Location)
			fmt.Println("          Type: " + ae.Type)
			fmt.Println("     MediaType: " + ae.MediaType)
			fmt.Println("     CreatedAt: " + ae.CreatedAt.String())
			fmt.Println()
			return nil
		})
		return nil
	},
}
