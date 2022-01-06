module github.com/google/go-containerregistry/pkg/authn/k8schain

go 1.14

require (
	github.com/google/go-containerregistry v0.8.0
	github.com/vdemeester/k8s-pkg-credentialprovider v1.21.0-1
	k8s.io/api v0.21.1
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v0.21.1
)

replace github.com/google/go-containerregistry => ../../../
