module github.com/google/go-containerregistry

go 1.13

require (
	github.com/Microsoft/go-winio v0.4.14 // indirect
	github.com/containerd/containerd v1.3.0 // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/docker/cli v0.0.0-20191017083524-a8ff7f821017
	github.com/docker/distribution v2.6.0-rc.1.0.20180327202408-83389a148052+incompatible // indirect
	github.com/docker/docker v1.4.2-0.20190924003213-a8608b5b67c7
	github.com/docker/docker-credential-helpers v0.6.3 // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/google/go-cmp v0.3.0
	github.com/googleapis/gnostic v0.2.2 // indirect
	github.com/gorilla/mux v1.7.3 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/maxbrunsfeld/counterfeiter/v6 v6.2.2
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/onsi/ginkgo v1.10.1 // indirect
	github.com/onsi/gomega v1.7.0 // indirect
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/sirupsen/logrus v1.4.2 // indirect
	github.com/spf13/cobra v0.0.5
	golang.org/x/crypto v0.0.0-20190510104115-cbcb75029529 // indirect
	golang.org/x/oauth2 v0.0.0-20190402181905-9f3314589c9a
	golang.org/x/sync v0.0.0-20190423024810-112230192c58
	golang.org/x/sys v0.0.0-20191010194322-b09406accb47 // indirect
	golang.org/x/time v0.0.0-20180412165947-fbb02b2291d2 // indirect
	golang.org/x/tools v0.0.0-20191205215504-7b8c8591a921 // indirect
	google.golang.org/grpc v1.24.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/api v0.15.7
	k8s.io/apimachinery v0.15.7
	k8s.io/client-go v0.15.7
	k8s.io/code-generator v0.15.7
	k8s.io/kubernetes v1.15.7
)

replace (
	k8s.io/api => k8s.io/api v0.15.7
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.15.7
	k8s.io/apimachinery => k8s.io/apimachinery v0.15.8-beta.1
	k8s.io/apiserver => k8s.io/apiserver v0.15.7
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.15.7
	k8s.io/client-go => k8s.io/client-go v0.15.7
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.15.7
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.15.7
	k8s.io/code-generator => k8s.io/code-generator v0.15.8-beta.1
	k8s.io/component-base => k8s.io/component-base v0.15.7
	k8s.io/cri-api => k8s.io/cri-api v0.15.7
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.15.7
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.15.7
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.15.7
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.15.7
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.15.7
	k8s.io/kubelet => k8s.io/kubelet v0.15.7
	k8s.io/kubernetes => k8s.io/kubernetes v1.15.7
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.15.7
	k8s.io/metrics => k8s.io/metrics v0.15.7
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.15.7
)
