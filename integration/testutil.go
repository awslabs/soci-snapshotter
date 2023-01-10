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

package integration

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/csv"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	shell "github.com/awslabs/soci-snapshotter/util/dockershell"
	"github.com/awslabs/soci-snapshotter/util/dockershell/compose"
	dexec "github.com/awslabs/soci-snapshotter/util/dockershell/exec"
	"github.com/awslabs/soci-snapshotter/util/testutil"
	"github.com/containerd/containerd/platforms"
	spec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/rs/xid"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultContainerdConfigPath  = "/etc/containerd/config.toml"
	defaultSnapshotterConfigPath = "/etc/soci-snapshotter-grpc/config.toml"
	builtinSnapshotterFlagEnv    = "BUILTIN_SNAPSHOTTER"
	buildArgsEnv                 = "DOCKER_BUILD_ARGS"
	dockerLibrary                = "public.ecr.aws/docker/library/"
)

const (
	alpineImage   = "alpine:latest"
	nginxImage    = "nginx:latest"
	ubuntuImage   = "ubuntu:latest"
	drupalImage   = "drupal:latest"
	rabbitmqImage = "rabbitmq:latest"
)

const proxySnapshotterConfig = `
[proxy_plugins]
  [proxy_plugins.soci]
    type = "snapshot"
    address = "/run/soci-snapshotter-grpc/soci-snapshotter-grpc.sock"
`

func trimSha256Prefix(s string) string {
	return strings.TrimPrefix(s, "sha256:")
}

func isTestingBuiltinSnapshotter() bool {
	return os.Getenv(builtinSnapshotterFlagEnv) == "true"
}

func getBuildArgsFromEnv(t *testing.T) []string {
	buildArgsStr := os.Getenv(buildArgsEnv)
	if buildArgsStr == "" {
		return nil
	}
	r := csv.NewReader(strings.NewReader(buildArgsStr))
	buildArgs, err := r.Read()
	if err != nil {
		t.Fatalf("failed to get build args from env %v", buildArgsEnv)
	}
	return buildArgs
}

func isFileExists(sh *shell.Shell, file string) bool {
	return sh.Command("test", "-f", file).Run() == nil
}

func isDirExists(sh *shell.Shell, dir string) bool {
	return sh.Command("test", "-d", dir).Run() == nil
}

type imageOpt func(*imageInfo)

func withPlatform(p spec.Platform) imageOpt {
	return func(i *imageInfo) {
		i.platform = p
	}
}

type imageInfo struct {
	ref       string
	creds     string
	plainHTTP bool
	platform  spec.Platform
}

func dockerhub(name string, opts ...imageOpt) imageInfo {
	i := imageInfo{dockerLibrary + name, "", false, platforms.DefaultSpec()}
	for _, opt := range opts {
		opt(&i)
	}
	return i
}

type registryConfig struct {
	host      string
	user      string
	pass      string
	port      int
	credstr   string
	plainHTTP bool
}

type registryConfigOpt func(*registryConfig)

func withPort(port int) registryConfigOpt {
	return func(rc *registryConfig) {
		rc.port = port
	}
}

func withCreds(creds string) registryConfigOpt {
	return func(rc *registryConfig) {
		rc.credstr = creds
	}
}

func withPlainHTTP() registryConfigOpt {
	return func(rc *registryConfig) {
		rc.plainHTTP = true
	}
}

func newRegistryConfig(opts ...registryConfigOpt) registryConfig {
	rc := registryConfig{
		host: fmt.Sprintf("registry-%s.test", xid.New().String()),
		user: "dummyuser",
		pass: "dummypass",
	}
	rc.credstr = rc.user + ":" + rc.pass
	for _, opt := range opts {
		opt(&rc)
	}
	return rc
}

func (c registryConfig) hostWithPort() string {
	if c.port != 0 {
		return fmt.Sprintf("%s:%d", c.host, c.port)
	}
	return c.host
}

func (c registryConfig) creds() string {
	return c.credstr
}

func (c registryConfig) mirror(imageName string, opts ...imageOpt) imageInfo {
	i := imageInfo{c.hostWithPort() + "/" + imageName, c.creds(), c.plainHTTP, platforms.DefaultSpec()}
	for _, opt := range opts {
		opt(&i)
	}
	return i
}

type registryOptions struct {
	network string
}

type registryOpt func(o *registryOptions)

func newShellWithRegistry(t *testing.T, r registryConfig, opts ...registryOpt) (sh *shell.Shell, done func() error) {
	var rOpts registryOptions
	for _, o := range opts {
		o(&rOpts)
	}
	var (
		pRoot       = testutil.GetProjectRoot(t)
		caCertDir   = "/usr/local/share/ca-certificates"
		serviceName = "testing"
	)

	// Setup dummy creds for test
	crt, key, err := generateRegistrySelfSignedCert(r.host)
	if err != nil {
		t.Fatalf("failed to generate cert: %v", err)
	}
	htpasswd, err := generateBasicHtpasswd(r.user, r.pass)
	if err != nil {
		t.Fatalf("failed to generate htpasswd: %v", err)
	}
	authDir, err := os.MkdirTemp("", "tmpcontext")
	if err != nil {
		t.Fatalf("failed to prepare auth tmpdir")
	}
	if err := os.WriteFile(filepath.Join(authDir, "domain.key"), key, 0666); err != nil {
		t.Fatalf("failed to prepare key file")
	}
	if err := os.WriteFile(filepath.Join(authDir, "domain.crt"), crt, 0666); err != nil {
		t.Fatalf("failed to prepare crt file")
	}
	if err := os.WriteFile(filepath.Join(authDir, "htpasswd"), htpasswd, 0666); err != nil {
		t.Fatalf("failed to prepare htpasswd file")
	}

	targetStage := "containerd-snapshotter-base"
	if isTestingBuiltinSnapshotter() {
		targetStage = "containerd-snapshotter-base"
	}
	// Run testing environment on docker compose
	cOpts := []compose.Option{
		compose.WithBuildArgs(getBuildArgsFromEnv(t)...),
		compose.WithStdio(testutil.TestingLogDest()),
	}
	networkConfig := ""
	var cleanups []func() error
	if nw := rOpts.network; nw != "" {
		done, err := dexec.NewTempNetwork(nw)
		if err != nil {
			t.Fatalf("failed to create temp network %v: %v", nw, err)
		}
		cleanups = append(cleanups, done)
		networkConfig = fmt.Sprintf(`
networks:
  default:
    external:
      name: %s
`, nw)
	}
	c, err := compose.New(testutil.ApplyTextTemplate(t, `
version: "3.7"
services:
  {{.ServiceName}}:
    build:
      context: {{.ImageContextDir}}
      target: {{.TargetStage}}
      args:
      - SNAPSHOTTER_BUILD_FLAGS="-race"
    privileged: true
    init: true
    entrypoint: [ "sleep", "infinity" ]
    environment:
    - NO_PROXY=127.0.0.1,localhost,{{.RegistryHost}}:443
    tmpfs:
    - /tmp:exec,mode=777
    - /var/lib/containerd
    - /var/lib/soci-snapshotter-grpc
    volumes:
    - /dev/fuse:/dev/fuse
  registry:
    image: ghcr.io/oci-playground/registry:v3.0.0-alpha.1
    container_name: {{.RegistryHost}}
    environment:
    - REGISTRY_AUTH=htpasswd
    - REGISTRY_AUTH_HTPASSWD_REALM="Registry Realm"
    - REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd
    - REGISTRY_HTTP_TLS_CERTIFICATE=/auth/domain.crt
    - REGISTRY_HTTP_TLS_KEY=/auth/domain.key
    - REGISTRY_HTTP_ADDR={{.RegistryHost}}:443
    volumes:
    - {{.AuthDir}}:/auth:ro
{{.NetworkConfig}}
`, struct {
		ServiceName     string
		ImageContextDir string
		TargetStage     string
		RegistryHost    string
		AuthDir         string
		NetworkConfig   string
	}{
		ServiceName:     serviceName,
		ImageContextDir: pRoot,
		TargetStage:     targetStage,
		RegistryHost:    r.host,
		AuthDir:         authDir,
		NetworkConfig:   networkConfig,
	}), cOpts...)
	if err != nil {
		t.Fatalf("failed to prepare compose: %v", err)
	}
	de, ok := c.Get(serviceName)
	if !ok {
		t.Fatalf("failed to get shell of service %v", serviceName)
	}
	sh = shell.New(de, testutil.NewTestingReporter(t))

	// Install cert and login to the registry
	if err := testutil.WriteFileContents(sh, filepath.Join(caCertDir, "domain.crt"), crt, 0600); err != nil {
		t.Fatalf("failed to write cert at %v: %v", caCertDir, err)
	}
	sh.
		X("update-ca-certificates").
		Retry(100, "nerdctl", "login", "-u", r.user, "-p", r.pass, r.host)
	return sh, func() error {
		if err := c.Cleanup(); err != nil {
			return err
		}
		for _, f := range cleanups {
			if err := f(); err != nil {
				return err
			}
		}
		return os.RemoveAll(authDir)
	}
}

func newSnapshotterBaseShell(t *testing.T) (*shell.Shell, func() error) {
	var (
		pRoot       = testutil.GetProjectRoot(t)
		serviceName = "testing"
	)
	targetStage := "containerd-snapshotter-base"
	c, err := compose.New(testutil.ApplyTextTemplate(t, `
version: "3.7"
services:
  {{.ServiceName}}:
    build:
      context: {{.ImageContextDir}}
      target: {{.TargetStage}}
      args:
      - SNAPSHOTTER_BUILD_FLAGS="-race"
    privileged: true
    init: true
    entrypoint: [ "sleep", "infinity" ]
    environment:
    - NO_PROXY=127.0.0.1,localhost
    tmpfs:
    - /tmp:exec,mode=777
    - /var/lib/containerd
    - /var/lib/soci-snapshotter-grpc
    volumes:
    - /dev/fuse:/dev/fuse
`, struct {
		ServiceName     string
		ImageContextDir string
		TargetStage     string
	}{
		ServiceName:     serviceName,
		ImageContextDir: pRoot,
		TargetStage:     targetStage,
	}),
		compose.WithBuildArgs(getBuildArgsFromEnv(t)...),
		compose.WithStdio(testutil.TestingLogDest()),
	)
	if err != nil {
		t.Fatalf("failed to prepare compose: %v", err)
	}
	de, ok := c.Get(serviceName)
	if !ok {
		t.Fatalf("failed to get shell of service %v", serviceName)
	}
	sh := shell.New(de, testutil.NewTestingReporter(t))
	if !isTestingBuiltinSnapshotter() {
		if err := testutil.WriteFileContents(sh, defaultContainerdConfigPath, []byte(proxySnapshotterConfig), 0600); err != nil {
			t.Fatalf("failed to wrtie containerd config %v: %v", defaultContainerdConfigPath, err)
		}
	}
	return sh, c.Cleanup
}

func generateRegistrySelfSignedCert(registryHost string) (crt, key []byte, _ error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 60)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, err
	}
	template := x509.Certificate{
		IsCA:                  true,
		BasicConstraintsValid: true,
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{CommonName: registryHost},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              []string{registryHost},
	}
	privatekey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	publickey := &privatekey.PublicKey
	cert, err := x509.CreateCertificate(rand.Reader, &template, &template, publickey, privatekey)
	if err != nil {
		return nil, nil, err
	}
	certPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert})
	privBytes, err := x509.MarshalPKCS8PrivateKey(privatekey)
	if err != nil {
		return nil, nil, err
	}
	keyPem := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})
	return certPem, keyPem, nil
}

func generateBasicHtpasswd(user, pass string) ([]byte, error) {
	bpass, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	return []byte(user + ":" + string(bpass) + "\n"), nil
}

func getManifestDigest(sh *shell.Shell, ref string, platform spec.Platform) (string, error) {
	buffer := new(bytes.Buffer)
	sh.Pipe(buffer, []string{"ctr", "image", "list", "name==" + ref}, []string{"awk", `NR==2{printf "%s", $3}`})
	content := sh.O("ctr", "content", "get", buffer.String())
	var index spec.Index
	err := json.Unmarshal(content, &index)
	if err != nil {
		return "", err
	}
	matcher := platforms.OnlyStrict(platform)
	for _, desc := range index.Manifests {
		if matcher.Match(*desc.Platform) {
			return desc.Digest.String(), nil
		}
	}
	return "", fmt.Errorf("could not find manifest for %s for platform %s", ref, platforms.Format(platform))
}

func genRandomByteData(size int64) []byte {
	b := make([]byte, size)
	rand.Read(b)
	return b
}
