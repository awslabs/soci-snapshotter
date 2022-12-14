module github.com/awslabs/soci-snapshotter

go 1.17

require (
	github.com/containerd/containerd v1.6.8
	github.com/containerd/continuity v0.2.2
	github.com/docker/cli v20.10.11+incompatible
	github.com/docker/go-metrics v0.0.1
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da
	github.com/google/flatbuffers v2.0.8+incompatible
	github.com/google/go-cmp v0.5.6
	github.com/google/uuid v1.2.0
	github.com/hanwen/go-fuse/v2 v2.1.1-0.20210825171523-3ab5d95a30ae
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/go-retryablehttp v0.7.1
	github.com/moby/sys/mountinfo v0.5.0
	github.com/montanaflynn/stats v0.6.6
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.0-rc2
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.1
	github.com/rs/xid v1.3.0
	github.com/sirupsen/logrus v1.8.1
	go.etcd.io/bbolt v1.3.6
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5
	golang.org/x/sync v0.1.0
	golang.org/x/sys v0.0.0-20220412211240-33da011f77ad
	google.golang.org/grpc v1.43.0
	k8s.io/api v0.23.0
	k8s.io/apimachinery v0.23.0
	k8s.io/client-go v0.23.0
	k8s.io/cri-api v0.24.0-alpha.0
	oras.land/oras-go/v2 v2.0.0-rc.4
)

require (
	github.com/Microsoft/go-winio v0.5.1 // indirect
	github.com/Microsoft/hcsshim v0.9.4 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/containerd/cgroups v1.0.3 // indirect
	github.com/containerd/fifo v1.0.0 // indirect
	github.com/containerd/ttrpc v1.1.0 // indirect
	github.com/containerd/typeurl v1.0.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/docker v20.10.7+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.6.4 // indirect
	github.com/docker/go-events v0.0.0-20190806004212-e31b211e4f1c // indirect
	github.com/go-logr/logr v1.2.2 // indirect
	github.com/gogo/googleapis v1.4.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/googleapis/gnostic v0.5.5 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.1 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.13.6 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/moby/locker v1.0.1 // indirect
	github.com/moby/sys/signal v0.6.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/onsi/ginkgo v1.16.4 // indirect
	github.com/onsi/gomega v1.17.0 // indirect
	github.com/opencontainers/runc v1.1.2 // indirect
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417 // indirect
	github.com/opencontainers/selinux v1.10.1 // indirect
	github.com/pelletier/go-toml v1.9.4 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.30.0 // indirect
	github.com/prometheus/procfs v0.7.3 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.opencensus.io v0.23.0 // indirect
	golang.org/x/net v0.0.0-20211216030914-fe4d6282115f // indirect
	golang.org/x/oauth2 v0.0.0-20210819190943-2bc19b11175f // indirect
	golang.org/x/term v0.0.0-20210615171337-6886f2dfbf5b // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/time v0.0.0-20210723032227-1f47c861a9ac // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20211208223120-3a66f561d7aa // indirect
	google.golang.org/protobuf v1.27.1 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	gotest.tools/v3 v3.0.3 // indirect
	k8s.io/klog/v2 v2.30.0 // indirect
	k8s.io/kube-openapi v0.0.0-20211115234752-e816edb12b65 // indirect
	k8s.io/utils v0.0.0-20210930125809-cb0fa318a74b // indirect
	sigs.k8s.io/json v0.0.0-20211020170558-c049b76a60c6 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.1.2 // indirect
	sigs.k8s.io/yaml v1.2.0 // indirect
)

// Temporary fork for avoiding importing patent-protected code: https://github.com/hashicorp/golang-lru/issues/73
replace github.com/hashicorp/golang-lru => github.com/ktock/golang-lru v0.5.5-0.20211029085301-ec551be6f75c
