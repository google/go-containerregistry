# `authconfig`

<img src="../../images/crane.png" width="40%">

This tool generates the auth config which includes a credential for the given registry 
server using the credential helpers provided by the cloud platforms (e.g., GKE, EKS, or AKS)
Like [`krane`](../krane/README.md), this tool retrieves the credential by authenticating 
with common "workload identity" mechanisms on such platforms.

The following script provides the same functionality with `krane $REGISTRY_SERVER/$REPOSITORY/$IMAGE_NAME:$TAG`.
```bash
AUTH_CONFIG=$(authconfig --server $REGISTRY_SERVER)
echo "{ \"auths\": {\"$REGISTRY_SERVER\":$AUTH_CONFIG}}" > docker_config.json
DOCKER_CONFIG=docker_config.json crane $REGISTRY_SERVER/$REPOSITORY/$IMAGE_NAME:$TAG
```

Note that this tool can be utilized for avoiding the tight coupling between platform specific codes 
- such as [ECR helper](https://github.com/awslabs/amazon-ecr-credential-helper) - and common 
docker related codes.