module github.com/google/go-containerregistry/pkg/authn/k8schain

go 1.14

require (
	github.com/golang/protobuf v1.4.3 // indirect
	github.com/google/go-containerregistry v0.4.0
	github.com/google/uuid v1.1.2 // indirect
	github.com/onsi/gomega v1.10.3 // indirect
	github.com/vdemeester/k8s-pkg-credentialprovider v1.19.7
	golang.org/x/sys v0.0.0-20210124154548-22da62e12c0c // indirect
	google.golang.org/protobuf v1.25.0 // indirect
	gopkg.in/check.v1 v1.0.0-20200902074654-038fdea0a05b // indirect
	k8s.io/api v0.19.7
	k8s.io/apimachinery v0.19.7
	k8s.io/client-go v0.19.7
	k8s.io/klog/v2 v2.5.0 // indirect
	k8s.io/kube-openapi v0.0.0-20201113171705-d219536bb9fd // indirect
)

replace github.com/google/go-containerregistry => ../../..
