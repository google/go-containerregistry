# `crane`

[`crane`](doc/crane.md) is a tool for interacting with remote images
and registries.

## Installation

```
GO111MODULE=on go get -u github.com/google/go-containerregistry/cmd/crane
```

## Images

You can also use crane as docker image

```sh
$ docker run --rm gcr.io/go-containerregistry/crane ls ubuntu

2019/12/03 09:33:01 No matching credentials were found, falling back on anonymous
10.04
12.04.5
12.04
12.10
```

And it's also available with a shell, which uses the `debug` tag

```sh
docker run --rm -it --entrypoint "/busybox/sh" gcr.io/go-containerregistry/crane:debug
```

### Using with GitLab

```yaml
# Tags an existing Docker image which was tagged with the short commit hash with the tag 'latest'
docker-tag-latest:
  stage: latest
  only:
    refs:
      - master
  image:
    name: gcr.io/go-containerregistry/crane:debug
    entrypoint: [""]
  script:
    - crane auth login -u $CI_REGISTRY_USER -p $CI_REGISTRY_PASSWORD $CI_REGISTRY
    - crane tag $CI_REGISTRY_IMAGE:$CI_COMMIT_SHORT_SHA latest
```
