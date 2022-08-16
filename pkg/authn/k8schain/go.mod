module github.com/google/go-containerregistry/pkg/authn/k8schain

go 1.17

replace (
	github.com/google/go-containerregistry => ../../../
	github.com/google/go-containerregistry/pkg/authn/kubernetes => ../kubernetes/
)

require (
	github.com/awslabs/amazon-ecr-credential-helper/ecr-login v0.0.0-20220802171026-617dc7abb2ea
	github.com/chrismellard/docker-credential-acr-env v0.0.0-20220327082430-c57b701bfc08
	github.com/google/go-containerregistry v0.11.0
	github.com/google/go-containerregistry/pkg/authn/kubernetes v0.0.0-20220808030257-7196cf3dc436
	k8s.io/api v0.24.3
	k8s.io/client-go v0.24.3
)

require (
	cloud.google.com/go/compute v1.8.0 // indirect
	github.com/Azure/azure-sdk-for-go v66.0.0+incompatible // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.28 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.21 // indirect
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.11 // indirect
	github.com/Azure/go-autorest/autorest/azure/cli v0.4.6 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/logger v0.2.1 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/aws/aws-sdk-go-v2 v1.16.11 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.17.1 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.12.14 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.12.12 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.18 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.4.12 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.19 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecr v1.17.12 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecrpublic v1.13.12 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.9.12 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.11.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.16.13 // indirect
	github.com/aws/smithy-go v1.12.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dimchansky/utfbom v1.1.1 // indirect
	github.com/docker/cli v20.10.17+incompatible // indirect
	github.com/docker/distribution v2.8.1+incompatible // indirect
	github.com/docker/docker v20.10.17+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.6.4 // indirect
	github.com/emicklei/go-restful/v3 v3.9.0 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.20.0 // indirect
	github.com/go-openapi/swag v0.22.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.4.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/gnostic v0.6.9 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.3-0.20220729202839-6ad7100eb087 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/sirupsen/logrus v1.9.0 // indirect
	golang.org/x/crypto v0.0.0-20220722155217-630584e8d5aa // indirect
	golang.org/x/net v0.0.0-20220812174116-3211cb980234 // indirect
	golang.org/x/oauth2 v0.0.0-20220808172628-8227340efae7 // indirect
	golang.org/x/sync v0.0.0-20220722155255-886fb9371eb4 // indirect
	golang.org/x/sys v0.0.0-20220811171246-fbc7d0a398ab // indirect
	golang.org/x/term v0.0.0-20220722155259-a9ba230a4035 // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/time v0.0.0-20220722155302-e5dcc9cfc0b9 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apimachinery v0.24.3 // indirect
	k8s.io/klog/v2 v2.70.1 // indirect
	k8s.io/kube-openapi v0.0.0-20220803164354-a70c9af30aea // indirect
	k8s.io/utils v0.0.0-20220812165043-ad590609e2e5 // indirect
	sigs.k8s.io/json v0.0.0-20220713155537-f223a00ba0e2 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)
