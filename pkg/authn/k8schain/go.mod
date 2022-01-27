module github.com/google/go-containerregistry/pkg/authn/k8schain

go 1.14

require (
	github.com/Azure/azure-sdk-for-go v61.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.11 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.12.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecr v1.13.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecrpublic v1.10.0 // indirect
	github.com/awslabs/amazon-ecr-credential-helper/ecr-login v0.0.0-20211215200129-69c85dc22db6
	github.com/chrismellard/docker-credential-acr-env v0.0.0-20220119192733-fe33c00cee21
	github.com/google/go-containerregistry v0.8.1-0.20220110151055-a61fd0a8e2bb
	github.com/google/go-containerregistry/pkg/authn/kubernetes v0.0.0-20220110151055-a61fd0a8e2bb
	golang.org/x/crypto v0.0.0-20220112180741-5e0467b6c7ce // indirect
	k8s.io/api v0.22.5
	k8s.io/client-go v0.22.5
)

replace (
	github.com/google/go-containerregistry => ../../../
	github.com/google/go-containerregistry/pkg/authn/kubernetes => ../kubernetes/
)
