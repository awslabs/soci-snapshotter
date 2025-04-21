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
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/awslabs/soci-snapshotter/config"
	commonmetrics "github.com/awslabs/soci-snapshotter/fs/metrics/common"
	"github.com/awslabs/soci-snapshotter/soci/store"
	shell "github.com/awslabs/soci-snapshotter/util/dockershell"
	"github.com/awslabs/soci-snapshotter/util/dockershell/compose"
	dexec "github.com/awslabs/soci-snapshotter/util/dockershell/exec"
	"github.com/awslabs/soci-snapshotter/util/testutil"
	"github.com/containerd/containerd/images"
	"github.com/containerd/platforms"
	"github.com/opencontainers/go-digest"
	spec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pelletier/go-toml"
	"github.com/rs/xid"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultContainerdConfigPath  = "/etc/containerd/config.toml"
	defaultSnapshotterConfigPath = "/etc/soci-snapshotter-grpc/config.toml"
	builtinSnapshotterFlagEnv    = "BUILTIN_SNAPSHOTTER"
	buildArgsEnv                 = "DOCKER_BUILD_ARGS"
	dockerLibrary                = "public.ecr.aws/docker/library/"
	// Registry images to use in the test infrastructure. These are not intended to be used
	// as images in the test itself, but just when we're setting up docker compose.
	oci10RegistryImage = "registry:soci_test"
	oci11RegistryImage = "public.ecr.aws/soci-workshop-examples/zot:v2.0.3-" + runtime.GOARCH
)

// Commonly used CLI commands
var (
	runSociCmd   = []string{"nerdctl", "run", "--pull", "never", "--net", "none", "--snapshotter", "soci"}
	imagePullCmd = []string{"nerdctl", "pull", "--snapshotter", "soci"}
)

// These are images that we use in our integration tests
const (
	helloImage    = "hello-world:latest"
	alpineImage   = "alpine:3.17.1"
	nginxImage    = "nginx:1.23.3"
	ubuntuImage   = "ubuntu:23.04"
	drupalImage   = "drupal:10.0.2"
	rabbitmqImage = "rabbitmq:3.11.7"
	// Pinned version of the cloudwatch agent x86 image that points to a single image manifest
	cloudwatchAgentx86Image    = "cloudwatch-agent:1.300053.0b1046-amd64"
	cloudwatchAgentx86ImageRef = "public.ecr.aws/cloudwatch-agent/" + cloudwatchAgentx86Image
	// Pinned version of rabbitmq that points to a multi architecture index.
	pinnedRabbitmqImage = "rabbitmq@sha256:19e69a7a65fa6b1d0a5c658bad8ec03d2c9900a98ebbc744c34d49179ff517bf"
	// These 2 images enable us to test cases where 2 different images
	// have shared layers (thus shared ztocs if built with the same parameters).
	nginxAlpineImage  = "nginx:1.22-alpine3.17"
	nginxAlpineImage2 = "nginx:1.23-alpine3.17"
)

const proxySnapshotterConfig = `
[proxy_plugins]
  [proxy_plugins.soci]
    type = "snapshot"
    address = "/run/soci-snapshotter-grpc/soci-snapshotter-grpc.sock"
	capabilities = ["multi-remap-ids", "remap-ids"]
`

const containerdConfigTemplate = `
version = 2

disabled_plugins = [
	"io.containerd.snapshotter.v1.aufs",
	"io.containerd.snapshotter.v1.btrfs",
	"io.containerd.snapshotter.v1.devmapper",
	"io.containerd.snapshotter.v1.zfs",
	"io.containerd.tracing.processor.v1.otlp",
	"io.containerd.internal.v1.tracing",
	"io.containerd.grpc.v1.cri",
]

[plugins."io.containerd.snapshotter.v1.soci"]
root_path = "/var/lib/soci-snapshotter-grpc/"
disable_verification = {{.DisableVerification}}

[plugins."io.containerd.snapshotter.v1.soci".blob]
check_always = true

[debug]
format = "json"
level = "{{.LogLevel}}"

{{.AdditionalConfig}}
`

type composeDefaultTemplateArgs struct {
	Entrypoint      string
	ImageContextDir string
}

type composeDefaultTemplateOpt func(*composeDefaultTemplateArgs)

func withEntrypoint(entrypoint string) composeDefaultTemplateOpt {
	return func(args *composeDefaultTemplateArgs) {
		args.Entrypoint = entrypoint
	}
}

const composeDefaultTemplate = `
services:
  testing:
   image: soci_base:soci_test
   privileged: true
   entrypoint: {{.Entrypoint}}
   environment:
    - NO_PROXY=127.0.0.1,localhost
    - GOCOVERDIR=/test_coverage
   tmpfs:
    - /tmp:exec,mode=777
    - /var/lib/containerd
    - /var/lib/soci-snapshotter-grpc
   volumes:
    - /dev/fuse:/dev/fuse
    - {{.ImageContextDir}}/cov/integration:/test_coverage
`
const composeRegistryTemplate = `
services:
 {{.ServiceName}}:
  image: soci_base:soci_test
  privileged: true
  init: true
  entrypoint: [ "/integ_entrypoint.sh" ]
  environment:
   - NO_PROXY=127.0.0.1,localhost,{{.RegistryHost}}:443
   - GOCOVERDIR=/test_coverage
  tmpfs:
   - /tmp:exec,mode=777
   - /var/lib/containerd
   - /var/lib/soci-snapshotter-grpc
  volumes:
   - /dev/fuse:/dev/fuse
   - {{.ImageContextDir}}/cov/integration:/test_coverage
 registry:
  image: {{.RegistryImageRef}}
  container_name: {{.RegistryHost}}
  environment:
   - REGISTRY_AUTH=htpasswd
   - REGISTRY_AUTH_HTPASSWD_REALM="Registry Realm"
   - REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd
   - REGISTRY_HTTP_TLS_CERTIFICATE=/auth/domain.crt
   - REGISTRY_HTTP_TLS_KEY=/auth/domain.key
   - REGISTRY_HTTP_ADDR={{.RegistryHost}}:443
   - REGISTRY_STORAGE_DELETE_ENABLED=true
  volumes:
   - {{.HostVolumeMount}}/auth:/auth:ro
   - {{.HostVolumeMount}}/etc/zot/config.json:/etc/zot/config.json:ro
{{.NetworkConfig}}
`
const composeRegistryAltTemplate = `
services:
  {{.ServiceName}}:
    image: soci_base:soci_test
    privileged: true
    init: true
    entrypoint: [ "/integ_entrypoint.sh" ]
    environment:
    - NO_PROXY=127.0.0.1,localhost,{{.RegistryHost}}:443
    - GOCOVERDIR=/test_coverage
    tmpfs:
    - /tmp:exec,mode=777
    - /var/lib/containerd
    - /var/lib/soci-snapshotter-grpc
    volumes:
    - /dev/fuse:/dev/fuse
    - {{.ImageContextDir}}/cov/integration:/test_coverage
  registry:
    image: {{.RegistryImageRef}}
    container_name: {{.RegistryHost}}
    environment:
    - REGISTRY_AUTH=htpasswd
    - REGISTRY_AUTH_HTPASSWD_REALM="Registry Realm"
    - REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd
    - REGISTRY_HTTP_TLS_CERTIFICATE=/auth/domain.crt
    - REGISTRY_HTTP_TLS_KEY=/auth/domain.key
    - REGISTRY_HTTP_ADDR={{.RegistryHost}}:443
    - REGISTRY_STORAGE_DELETE_ENABLED=true
    volumes:
    - {{.HostVolumeMount}}/auth:/auth:ro
  registry-alt:
    image: {{.RegistryAltImageRef}}
    container_name: {{.RegistryAltHost}}
`

const composeBuildTemplate = `
services:
 {{.ServiceName}}:
  image: soci_base:soci_test
  build:
   context: {{.ImageContextDir}}
   target: {{.TargetStage}}
   args:
    - SNAPSHOTTER_BUILD_FLAGS="-race"
 registry:
  image: registry:soci_test
  build:
   context: {{.ImageContextDir}}
   target: {{.RegistryStage}}
`

const zotConfigTemplate = `
{
	"storage": {
		"rootDirectory": "/tmp/zot"
	},
	"http": {
		"address": "{{.Address}}",
		"port": "443",
		"realm": "Registry Realm",
		"auth": {
			"htpasswd": {
				"path": "/auth/htpasswd"
			}
		},
		"tls": {
			"cert": "/auth/domain.crt",
			"key": "/auth/domain.key"
		}
	}
}
`

type dockerComposeYaml struct {
	ServiceName         string
	ImageContextDir     string
	TargetStage         string
	RegistryStage       string
	RegistryImageRef    string
	RegistryAltImageRef string
	RegistryHost        string
	RegistryAltHost     string
	HostVolumeMount     string
	NetworkConfig       string
}

type zotConfigStruct struct {
	Address string
}

// getContainerdConfigToml creates a containerd config yaml, by appending all
// `additionalConfigs` to the default `containerdConfigTemplate`.
func getContainerdConfigToml(t *testing.T, disableVerification bool, additionalConfigs ...string) string {
	if !isTestingBuiltinSnapshotter() {
		additionalConfigs = append(additionalConfigs, proxySnapshotterConfig)
	}
	s, err := testutil.ApplyTextTemplate(containerdConfigTemplate, struct {
		LogLevel            string
		DisableVerification bool
		AdditionalConfig    string
	}{
		LogLevel:            containerdLogLevel,
		DisableVerification: disableVerification,
		AdditionalConfig:    strings.Join(additionalConfigs, "\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

type snapshotterConfigOpt func(*config.Config)

func withTCPMetrics(cfg *config.Config) {
	cfg.MetricsAddress = tcpMetricsAddress
}

func withUnixMetrics(cfg *config.Config) {
	cfg.MetricsAddress = unixMetricsAddress
	cfg.MetricsNetwork = "unix"
}

func withMaxConcurrency(m int64) snapshotterConfigOpt {
	return func(c *config.Config) {
		c.ServiceConfig.FSConfig.MaxConcurrency = m
	}
}

func withDisableBgFetcher(cfg *config.Config) {
	cfg.ServiceConfig.FSConfig.BackgroundFetchConfig.Disable = true
}

func withMinLayerSizeConfig(minLayerSize int64) snapshotterConfigOpt {
	return func(c *config.Config) {
		c.ServiceConfig.SnapshotterConfig.MinLayerSize = minLayerSize
	}
}

func withFuseWaitDuration(i int64) snapshotterConfigOpt {
	return func(c *config.Config) {
		c.ServiceConfig.FSConfig.FuseMetricsEmitWaitDurationSec = i
	}
}

func getSnapshotterConfigToml(t *testing.T, opts ...snapshotterConfigOpt) string {
	// For integ tests, we intentionally don't initialize the config to simulate
	// a partially filled config like you might find in a real /etc/soci-snapshotter-grpc/config.toml
	config := config.Config{}
	for _, opt := range opts {
		opt(&config)
	}
	s, err := toml.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	return string(s)
}

func isTestingBuiltinSnapshotter() bool {
	return os.Getenv(builtinSnapshotterFlagEnv) == "true"
}

func getBuildArgsFromEnv() ([]string, error) {
	buildArgsStr := os.Getenv(buildArgsEnv)
	if buildArgsStr == "" {
		return nil, nil
	}
	r := csv.NewReader(strings.NewReader(buildArgsStr))
	buildArgs, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to get build args from env %v", buildArgsEnv)
	}
	return buildArgs, nil
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

// encodeImageInfoNerdctl assembles command line options for pulling or pushing an image using nerdctl
func encodeImageInfoNerdctl(ii ...imageInfo) [][]string {
	var opts [][]string
	for _, i := range ii {
		var o []string
		if i.plainHTTP {
			o = append(o, "--insecure-registry")
		}
		o = append(o, i.ref)
		opts = append(opts, o)
	}
	return opts
}

func copyImage(sh *shell.Shell, src, dst imageInfo) {
	opts := encodeImageInfoNerdctl(src, dst)
	sh.
		X(append([]string{"nerdctl", "pull", "-q", "--platform", platforms.Format(src.platform)}, opts[0]...)...).
		X("ctr", "i", "tag", src.ref, dst.ref).
		X(append([]string{"nerdctl", "push", "-q", "--platform", platforms.Format(src.platform)}, opts[1]...)...)
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
		host: fmt.Sprintf("%s-registry-%s.test", compose.TestContainerBaseName, xid.New().String()),
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
	network          string
	registryImageRef string
}

func defaultRegistryOptions() registryOptions {
	return registryOptions{
		network:          "",
		registryImageRef: oci10RegistryImage,
	}
}

type registryOpt func(o *registryOptions)

func withRegistryImageRef(ref string) registryOpt {
	return func(o *registryOptions) {
		o.registryImageRef = ref
	}
}

func newShellWithRegistry(t *testing.T, r registryConfig, opts ...registryOpt) (sh *shell.Shell, done func() error) {
	rOpts := defaultRegistryOptions()
	for _, o := range opts {
		o(&rOpts)
	}
	var (
		caCertDir   = "/usr/local/share/ca-certificates"
		serviceName = "testing"
	)

	pRoot, err := testutil.GetProjectRoot()
	if err != nil {
		t.Fatal(err)
	}

	// Setup dummy creds for test
	crt, key, err := generateRegistrySelfSignedCert(r.host)
	if err != nil {
		t.Fatalf("failed to generate cert: %v", err)
	}
	htpasswd, err := generateBasicHtpasswd(r.user, r.pass)
	if err != nil {
		t.Fatalf("failed to generate htpasswd: %v", err)
	}

	hostVolumeMount := t.TempDir()
	authDir := filepath.Join(hostVolumeMount, "auth")
	if err := os.Mkdir(authDir, 0777); err != nil {
		t.Fatalf("failed to create auth folder in tempdir: %v", err)
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

	buildArgs, err := getBuildArgsFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	// Run testing environment on docker compose
	cOpts := []compose.Option{
		compose.WithBuildArgs(buildArgs...),
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

	zotDir := filepath.Join(hostVolumeMount, "etc/zot")
	if err := os.MkdirAll(zotDir, 0777); err != nil {
		t.Fatalf("failed to create zot folder in tempdir: %v", err)
	}
	zotConfigFile, err := testutil.ApplyTextTemplate(zotConfigTemplate, zotConfigStruct{
		Address: r.host,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(hostVolumeMount, "etc/zot/config.json"), []byte(zotConfigFile), 0666); err != nil {
		t.Fatalf("failed to prepare config.json: %v", err)
	}

	s, err := testutil.ApplyTextTemplate(composeRegistryTemplate, dockerComposeYaml{
		ServiceName:      serviceName,
		ImageContextDir:  pRoot,
		RegistryHost:     r.host,
		RegistryImageRef: rOpts.registryImageRef,
		HostVolumeMount:  hostVolumeMount,
		NetworkConfig:    networkConfig,
	})
	if err != nil {
		t.Fatal(err)
	}
	c, err := compose.Up(s, cOpts...)
	if err != nil {
		t.Fatalf("failed to prepare compose: %v", err)
	}
	de, ok := c.Get(serviceName)
	if !ok {
		t.Fatalf("failed to get shell of service %v", serviceName)
	}
	sh = shell.New(de, testutil.NewTestingReporter(t))

	// Install cert and login to the registry
	crtPath := filepath.Join(caCertDir, "domain.crt")
	if err := testutil.WriteFileContents(sh, crtPath, crt, 0600); err != nil {
		t.Fatalf("failed to write cert at %v: %v", caCertDir, err)
	}
	sh.
		X("trust", "anchor", crtPath).
		Retry(100, "nerdctl", "login", "-u", r.user, "-p", r.pass, r.host)
	return sh, func() error {
		killErr := testutil.KillMatchingProcess(sh, "soci-snapshotter-grpc")
		if err = c.Cleanup(); err != nil {
			return errors.Join(killErr, err)
		}
		for _, f := range cleanups {
			if err := f(); err != nil {
				return errors.Join(killErr, err)
			}
		}
		return killErr
	}
}

func newSnapshotterBaseShell(t *testing.T, opts ...composeDefaultTemplateOpt) (*shell.Shell, func() error) {

	serviceName := "testing"
	buildArgs, err := getBuildArgsFromEnv()
	if err != nil {
		t.Fatal(err)
	}

	pRoot, err := testutil.GetProjectRoot()
	if err != nil {
		t.Fatal(err)
	}

	args := composeDefaultTemplateArgs{
		Entrypoint:      `[ "/integ_entrypoint.sh" ]`,
		ImageContextDir: pRoot,
	}
	for _, opt := range opts {
		opt(&args)
	}

	s, err := testutil.ApplyTextTemplate(composeDefaultTemplate, args)
	if err != nil {
		t.Fatal(err)
	}
	c, err := compose.Up(s, compose.WithBuildArgs(buildArgs...), compose.WithStdio(testutil.TestingLogDest()))
	if err != nil {
		t.Fatalf("failed to prepare compose: %v", err)
	}
	de, ok := c.Get(serviceName)
	if !ok {
		t.Fatalf("failed to get shell of service %v", serviceName)
	}
	sh := shell.New(de, testutil.NewTestingReporter(t))
	if !isTestingBuiltinSnapshotter() {
		if err := testutil.WriteFileContents(sh, defaultContainerdConfigPath, []byte(getContainerdConfigToml(t, false)), 0600); err != nil {
			t.Fatalf("failed to write containerd config %v: %v", defaultContainerdConfigPath, err)
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
		NotAfter:              time.Now().AddDate(1, 0, 0), // one year
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

func getImageDigest(sh *shell.Shell, ref string) string {
	buffer := new(bytes.Buffer)
	sh.Pipe(buffer, []string{"ctr", "image", "list", "name==" + ref}, []string{"awk", `NR==2{printf "%s", $3}`})
	return buffer.String()
}

func getManifestDigest(sh *shell.Shell, ref string, platform spec.Platform) (string, error) {
	content := sh.O("ctr", "content", "get", getImageDigest(sh, ref))
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

func getReferrers(sh *shell.Shell, regConfig registryConfig, imgName, digest string) (*spec.Index, error) {
	var index spec.Index
	output, err := sh.OLog("curl", "-u", regConfig.creds(), fmt.Sprintf("https://%s:443/v2/%s/referrers/%s", regConfig.host, imgName, digest))
	if err != nil {
		return nil, fmt.Errorf("failed to get referrers: %w", err)
	}
	// If the referrers API returns a 404, try the fallback.
	if strings.Contains(string(output), "404") {
		referrersTag := strings.Replace(digest, ":", "-", 1)
		output, err = sh.OLog("curl", "--header", fmt.Sprintf("Accept: %s, %s", spec.MediaTypeImageIndex, images.MediaTypeDockerSchema2ManifestList), "-u", regConfig.creds(),
			fmt.Sprintf("https://%s:443/v2/%s/manifests/%s", regConfig.host, imgName, referrersTag))
		if err != nil {
			return nil, fmt.Errorf("failed to get referrers: %w", err)
		}
	}
	err = json.Unmarshal(output, &index)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal index: %w", err)
	}
	return &index, nil
}

func rebootContainerd(t *testing.T, sh *shell.Shell, customContainerdConfig, customSnapshotterConfig string) *testutil.LogMonitor {
	var (
		containerdRoot    = "/var/lib/containerd"
		containerdStatus  = "/run/containerd/"
		snapshotterSocket = "/run/soci-snapshotter-grpc/soci-snapshotter-grpc.sock"
		snapshotterRoot   = "/var/lib/soci-snapshotter-grpc"
	)

	// cleanup directories
	err := testutil.KillMatchingProcess(sh, "containerd")
	if err != nil {
		sh.Fatal("failed to kill containerd: %v", err)
	}
	err = testutil.KillMatchingProcess(sh, "soci-snapshotter-grpc")
	if err != nil {
		sh.Fatal("failed to kill soci: %v", err)
	}
	removeDirContents(sh, containerdRoot)
	if isDirExists(sh, containerdStatus) {
		removeDirContents(sh, containerdStatus)
	}
	if isFileExists(sh, snapshotterSocket) {
		sh.X("rm", snapshotterSocket)
	}
	if snDir := filepath.Join(snapshotterRoot, "/snapshotter/snapshots"); isDirExists(sh, snDir) {
		sh.X("find", snDir, "-maxdepth", "1", "-mindepth", "1", "-type", "d",
			"-exec", "umount", "{}/fs", ";")
	}
	removeDirContents(sh, snapshotterRoot)

	// run containerd and snapshotter
	containerdCmds := shell.C("containerd", "--log-level", containerdLogLevel)
	if customContainerdConfig != "" {
		containerdCmds = addConfig(t, sh, customContainerdConfig, containerdCmds...)
	}
	sh.Gox(containerdCmds...)
	snapshotterCmds := shell.C("/usr/local/bin/soci-snapshotter-grpc", "--log-level", sociLogLevel,
		"--address", snapshotterSocket)
	if customSnapshotterConfig != "" {
		snapshotterCmds = addConfig(t, sh, customSnapshotterConfig, snapshotterCmds...)
	}
	outR, errR, err := sh.R(snapshotterCmds...)
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	reporter := testutil.NewTestingReporter(t)
	var m *testutil.LogMonitor = testutil.NewLogMonitor(reporter, outR, errR)

	if err = testutil.LogConfirmStartup(m); err != nil {
		t.Fatalf("snapshotter startup failed: %v", err)
	}

	// make sure containerd and soci-snapshotter-grpc are up-and-running
	sh.Retry(100, "ctr", "snapshots", "--snapshotter", "soci",
		"prepare", "connectiontest-dummy-"+xid.New().String(), "")

	sh.XLog("containerd", "--version")

	return m
}

func removeDirContents(sh *shell.Shell, dir string) {
	// `rm -rf Dir` directly sometimes causes failure, e.g.,
	// rm: cannot remove '/var/lib/containerd/': Device or resource busy.
	// this might be a mount issue.
	sh.X("find", dir+"/.", "!", "-name", ".", "-prune", "-exec", "rm", "-rf", "{}", "+")
}

func addConfig(t *testing.T, sh *shell.Shell, conf string, cmds ...string) []string {
	configPath := strings.TrimSpace(string(sh.O("mktemp")))
	if err := testutil.WriteFileContents(sh, configPath, []byte(conf), 0600); err != nil {
		t.Fatalf("failed to add config to %v: %v", configPath, err)
	}
	return append(cmds, "--config", configPath)
}

func checkOverlayFallbackCount(output string, expected int) error {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if !strings.Contains(line, commonmetrics.FuseMountFailureCount) {
			continue
		}
		var got int
		_, err := fmt.Sscanf(line, fmt.Sprintf(`soci_fs_operation_count{layer="",operation_type="%s"} %%d`, commonmetrics.FuseMountFailureCount), &got)
		if err != nil {
			return err
		}
		if got != expected {
			return fmt.Errorf("unexpected overlay fallbacks: got %d, expected %d", got, expected)
		}
		return nil
	}
	if expected != 0 {
		return fmt.Errorf("expected %d overlay fallbacks but got 0", expected)
	}
	return nil
}

// middleSizeLayerInfo finds a layer not the smallest or largest (if possible), returns index, size, and layer count
// It requires containerd to be running
func middleSizeLayerInfo(t *testing.T, sh *shell.Shell, image imageInfo) (int, int64, int) {
	sh.O("nerdctl", "pull", "-q", "--platform", platforms.Format(image.platform), image.ref)

	imageManifestDigest, err := getManifestDigest(sh, image.ref, image.platform)
	if err != nil {
		t.Fatalf("Failed to get manifest digest: %v", err)
	}
	dgst, err := digest.Parse(imageManifestDigest)
	if err != nil {
		t.Fatalf("Failed to parse manifest digest: %v", err)
	}
	imageManifestJSON, err := FetchContentByDigest(sh, store.ContainerdContentStoreType, dgst)
	if err != nil {
		t.Fatalf("Failed to fetch manifest: %v", err)
	}
	imageManifest := new(spec.Manifest)
	if err := json.Unmarshal(imageManifestJSON, imageManifest); err != nil {
		t.Fatalf("cannot unmarshal image manifest: %v", err)
	}

	snapshotSizes := make([]int64, 0)
	for _, layerBlob := range imageManifest.Layers {
		snapshotSizes = append(snapshotSizes, layerBlob.Size)
	}

	sort.Slice(snapshotSizes, func(i, j int) bool { return snapshotSizes[i] < snapshotSizes[j] })
	if snapshotSizes[0] == snapshotSizes[len(snapshotSizes)-1] {
		// This condition would almost certainly invalidate the expected behavior of the calling test
		t.Fatalf("all %v layers are the same size (%v) when seeking middle size layer", len(snapshotSizes), snapshotSizes[0])
	}
	middleIndex := len(snapshotSizes) / 2
	middleSize := snapshotSizes[middleIndex]
	if snapshotSizes[0] == middleSize {
		// if the middle is also the smallest, find the next larger layer
		for middleIndex < len(snapshotSizes)-1 && snapshotSizes[middleIndex] == middleSize {
			middleIndex++
		}
	} else {
		// find the lowest index that is the same size as the middle
		for middleIndex > 0 && snapshotSizes[middleIndex-1] == middleSize {
			middleIndex--
		}
	}

	return middleIndex, middleSize, len(snapshotSizes)
}

func fetchContentFromPath(sh *shell.Shell, path string) []byte {
	return sh.O("cat", path)
}

func fetchSociContentStoreContentByDigest(sh *shell.Shell, dgst digest.Digest) []byte {
	path := filepath.Join(store.DefaultSociContentStorePath, "blobs", dgst.Algorithm().String(), dgst.Encoded())
	return sh.O("cat", path)
}

func fetchContainerdContentStoreContentByDigest(sh *shell.Shell, dgst digest.Digest) []byte {
	return sh.O("ctr", "content", "get", dgst.String())
}

func FetchContentByDigest(sh *shell.Shell, contentStoreType store.ContentStoreType, dgst digest.Digest) ([]byte, error) {
	contentStoreType, err := store.CanonicalizeContentStoreType(contentStoreType)
	if err != nil {
		return nil, err
	}
	switch contentStoreType {
	case store.SociContentStoreType:
		return fetchSociContentStoreContentByDigest(sh, dgst), nil
	case store.ContainerdContentStoreType:
		return fetchContainerdContentStoreContentByDigest(sh, dgst), nil
	default:
		return nil, store.ErrUnknownContentStoreType(contentStoreType)
	}
}

func withContentStoreConfig(opts ...store.Option) snapshotterConfigOpt {
	return func(c *config.Config) {
		c.ServiceConfig.FSConfig.ContentStoreConfig = store.NewStoreConfig(opts...)
	}
}
