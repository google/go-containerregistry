module github.com/google/go-containerregistry/cmd/krane

go 1.17

replace (
	github.com/google/go-containerregistry => ../..
	github.com/google/go-containerregistry/pkg/authn/k8schain => ../../pkg/authn/k8schain
	github.com/google/go-containerregistry/pkg/authn/kubernetes => ../../pkg/authn/kubernetes
)

require (
	github.com/google/go-containerregistry v0.8.1-0.20220110151055-a61fd0a8e2bb
	github.com/google/go-containerregistry/pkg/authn/k8schain v0.0.0-20211223213658-2874338840a6
)

require (
	cloud.google.com/go v0.99.0 // indirect
	github.com/Azure/azure-sdk-for-go v46.4.0+incompatible // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.18 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.13 // indirect
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.2 // indirect
	github.com/Azure/go-autorest/autorest/azure/cli v0.4.1 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/logger v0.2.1 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/aws/aws-sdk-go-v2 v1.12.0 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.12.0 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.7.0 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.9.0 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.3 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.1.0 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecr v1.13.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecrpublic v1.10.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.6.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.8.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.13.0 // indirect
	github.com/aws/smithy-go v1.9.1 // indirect
	github.com/awslabs/amazon-ecr-credential-helper/ecr-login v0.0.0-20211215200129-69c85dc22db6 // indirect
	github.com/chrismellard/docker-credential-acr-env v0.0.0-20210203204924-09e2b5a8ac86 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.10.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dimchansky/utfbom v1.1.0 // indirect
	github.com/docker/cli v20.10.12+incompatible // indirect
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v20.10.12+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.6.4 // indirect
	github.com/form3tech-oss/jwt-go v3.2.3+incompatible // indirect
	github.com/go-logr/logr v1.2.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-cmp v0.5.6 // indirect
	github.com/google/go-containerregistry/pkg/authn/kubernetes v0.0.0-20220110151055-a61fd0a8e2bb // indirect
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/googleapis/gnostic v0.5.5 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.13.6 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.2-0.20211117181255-693428a734f5 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/spf13/cobra v1.3.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/vbatts/tar-split v0.11.2 // indirect
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5 // indirect
	golang.org/x/net v0.0.0-20211216030914-fe4d6282115f // indirect
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8 // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c // indirect
	golang.org/x/sys v0.0.0-20220111092808-5a964db01320 // indirect
	golang.org/x/term v0.0.0-20210615171337-6886f2dfbf5b // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/time v0.0.0-20210723032227-1f47c861a9ac // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	k8s.io/api v0.22.5 // indirect
	k8s.io/apimachinery v0.22.5 // indirect
	k8s.io/client-go v0.22.5 // indirect
	k8s.io/klog/v2 v2.30.0 // indirect
	k8s.io/utils v0.0.0-20210930125809-cb0fa318a74b // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.1.2 // indirect
	sigs.k8s.io/yaml v1.2.0 // indirect
)
