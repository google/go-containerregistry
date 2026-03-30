module github.com/google/go-containerregistry

// The go directive declares the minimum Go version required for this module.
//
// DO NOT change this version unless support for older Go versions is dropped
// or the module requires newer Go features. To update the version used for CI
// and releases, update the ".go-version" file at the root of this repository.
go 1.25.0

require (
	github.com/containerd/stargz-snapshotter/estargz v0.18.2
	github.com/docker/cli v29.3.1+incompatible
	github.com/docker/distribution v2.8.3+incompatible
	github.com/google/go-cmp v0.7.0
	github.com/klauspost/compress v1.18.5
	github.com/mitchellh/go-homedir v1.1.0
	github.com/moby/docker-image-spec v1.3.1
	github.com/moby/moby/api v1.54.0
	github.com/moby/moby/client v0.3.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.1
	github.com/spf13/cobra v1.10.2
	golang.org/x/oauth2 v0.36.0
	golang.org/x/sync v0.20.0
	golang.org/x/tools v0.43.0
)

require (
	cloud.google.com/go/compute/metadata v0.7.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/docker-credential-helpers v0.9.3 // indirect
	github.com/docker/go-connections v0.6.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/vbatts/tar-split v0.12.2 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.61.0 // indirect
	go.opentelemetry.io/otel v1.36.0 // indirect
	go.opentelemetry.io/otel/metric v1.36.0 // indirect
	go.opentelemetry.io/otel/trace v1.36.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/mod v0.34.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
)
