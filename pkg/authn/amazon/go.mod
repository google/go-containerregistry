module github.com/google/go-containerregistry/pkg/authn/amazon

go 1.17

require (
	github.com/awslabs/amazon-ecr-credential-helper/ecr-login v0.0.0-20211027214941-f15886b5ccdc
	github.com/google/go-containerregistry v0.8.0
)

replace github.com/google/go-containerregistry => ../../../

require (
	github.com/aws/aws-sdk-go-v2 v1.7.1 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.5.0 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.3.1 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.3.0 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.1.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecr v1.4.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecrpublic v1.4.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.2.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.3.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.6.0 // indirect
	github.com/aws/smithy-go v1.6.0 // indirect
	github.com/docker/cli v20.10.12+incompatible // indirect
	github.com/docker/docker v20.10.12+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.6.4 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	golang.org/x/sys v0.0.0-20211216021012-1d35b9e2eb4e // indirect
)
