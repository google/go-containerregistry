module github.com/google/go-containerregistry/pkg/authn/k8schain

go 1.14

require (
	github.com/awslabs/amazon-ecr-credential-helper/ecr-login v0.0.0-20211027214941-f15886b5ccdc
	github.com/chrismellard/docker-credential-acr-env v0.0.0-20210203204924-09e2b5a8ac86
	github.com/google/go-containerregistry v0.8.0
	github.com/google/go-containerregistry/pkg/authn/kubernetes v0.0.0-00010101000000-000000000000
	k8s.io/api v0.23.1
	k8s.io/apimachinery v0.23.1
	k8s.io/client-go v0.23.1
)

replace (
	github.com/google/go-containerregistry => ../../../
	github.com/google/go-containerregistry/pkg/authn/kubernetes => ../kubernetes/
)
