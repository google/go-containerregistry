# `crane`

[`crane`](doc/crane.md) is a tool for interacting with remote images
and registries.

<img src="../../images/crane.png" width="40%">

## Installation

Download [latest release](https://github.com/google/go-containerregistry/releases/latest).

Install manually:

```
go install github.com/google/go-containerregistry/cmd/crane
```

### Install via brew

If you're macOS user and using [Homebrew](https://brew.sh/), you can install via brew command:

```sh
$ brew install crane
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
      - main
  image:
    name: gcr.io/go-containerregistry/crane:debug
    entrypoint: [""]
  script:
    - crane auth login -u $CI_REGISTRY_USER -p $CI_REGISTRY_PASSWORD $CI_REGISTRY
    - crane tag $CI_REGISTRY_IMAGE:$CI_COMMIT_SHORT_SHA latest
```
