module github.com/google/go-containerregistry/pkg/authn/k8schain

go 1.14

require (
	github.com/google/go-containerregistry v0.5.2-0.20210609162550-f0ce2270b3b4
	github.com/vdemeester/k8s-pkg-credentialprovider v1.21.0-1
	k8s.io/api v0.21.1
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v0.21.1
)

replace (
	github.com/google/go-containerregistry => ../../..
	// This forces transitive deps to use a version of `image-spec` which addresses https://github.com/advisories/GHSA-77vh-xpmg-72qh
	github.com/opencontainers/image-spec => github.com/opencontainers/image-spec v1.0.2-0.20211117181255-693428a734f5
)
