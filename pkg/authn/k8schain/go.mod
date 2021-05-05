module github.com/google/go-containerregistry/pkg/authn/k8schain

go 1.14

require (
	github.com/google/go-containerregistry v0.4.1-0.20210128200529-19c2b639fab1
	github.com/onsi/gomega v1.10.3 // indirect
	github.com/vdemeester/k8s-pkg-credentialprovider v1.21.0-1
	golang.org/x/text v0.3.5 // indirect
	gopkg.in/check.v1 v1.0.0-20200902074654-038fdea0a05b // indirect
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 // indirect
	k8s.io/api v0.21.0
	k8s.io/apimachinery v0.21.0
	k8s.io/client-go v0.21.0
)

replace github.com/google/go-containerregistry => ../../..
