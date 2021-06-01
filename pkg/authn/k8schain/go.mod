module github.com/google/go-containerregistry/pkg/authn/k8schain

go 1.14

require (
	github.com/google/go-containerregistry v0.4.1-0.20210128200529-19c2b639fab1
	github.com/vdemeester/k8s-pkg-credentialprovider v1.21.0-1
	k8s.io/api v0.21.1
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v0.21.1
)

replace github.com/google/go-containerregistry => ../../..
