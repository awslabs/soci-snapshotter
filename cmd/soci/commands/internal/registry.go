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

/*
   Copyright The containerd Authors.

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
	"io"
	"strings"

	dockercliconfig "github.com/docker/cli/cli/config"
	"github.com/urfave/cli/v3"
)

var RegistryFlags = []cli.Flag{
	&cli.BoolFlag{
		Name:    "skip-verify",
		Aliases: []string{"k"},
		Usage:   "Skip SSL certificate validation",
	},
	&cli.BoolFlag{
		Name:  "plain-http",
		Usage: "Allow connections using plain HTTP",
	},
	&cli.StringFlag{
		Name:    "user",
		Aliases: []string{"u"},
		Usage:   "User[:password] Registry user and password",
	},
	&cli.StringFlag{
		Name:  "refresh",
		Usage: "Refresh token for authorization server",
	},
	&cli.StringFlag{
		Name: "hosts-dir",
		// compatible with "/etc/docker/certs.d"
		Usage: "Custom hosts configuration directory",
	},
	&cli.StringFlag{
		Name:  "tlscacert",
		Usage: "Path to TLS root CA",
	},
	&cli.StringFlag{
		Name:  "tlscert",
		Usage: "Path to TLS client certificate",
	},
	&cli.StringFlag{
		Name:  "tlskey",
		Usage: "Path to TLS client key",
	},
	&cli.BoolFlag{
		Name:  "http-dump",
		Usage: "Dump all HTTP request/responses when interacting with container registry",
	},
	&cli.BoolFlag{
		Name:  "http-trace",
		Usage: "Enable HTTP tracing for registry interactions",
	},
}

// ResolveCredentials resolves registry credentials from multiple sources:
// 1. CLI --user/-u flag (username:password)
// 2. Docker config file (~/.docker/config.json)
// 3. Empty credentials for public registries
func ResolveCredentials(cmd *cli.Command, hostname string) (username, password string) {
	if cmd.IsSet("user") {
		user := cmd.String("user")
		if i := strings.IndexByte(user, ':'); i > 0 {
			return user[:i], user[i+1:]
		}
		return user, ""
	}

	cf := dockercliconfig.LoadDefaultConfigFile(io.Discard)
	if cf.ContainsAuth() {
		if ac, err := cf.GetAuthConfig(hostname); err == nil {
			return ac.Username, ac.Password
		}
	}

	return "", ""
}
