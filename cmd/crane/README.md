# `crane`

[`crane`](doc/crane.md) is a tool for interacting with remote images
and registries.

<img src="../../images/crane.png" width="40%">

A collection of useful things you can do with `crane` is [here](recipes.md).

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

### Install on arch

If you're arch linux user and use [some AUR
helper](https://wiki.archlinux.org/index.php/AUR_helpers) you can install it with one of
your favourite command:

```sh
$ yay -S go-crane-bin
```

## Images

You can also use crane as docker image

```sh
$ docker run --rm gcr.io/go-containerregistry/crane ls ubuntu
10.04
12.04.5
12.04
12.10
```

And it's also available with a shell, at the `:debug` tag:

```sh
docker run --rm -it --entrypoint "/busybox/sh" gcr.io/go-containerregistry/crane:debug
```

Tagged debug images are available at `gcr.io/go-containerregistry/crane/debug:[tag]`.

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
