# `crane`

[`crane`](doc/crane.md) is a tool for interacting with remote images
and registries.

<img src="../../images/crane.png" width="40%">

A collection of useful things you can do with `crane` is [here](recipes.md).

## Installation

### Install from Releases

1. Get the [latest release](https://github.com/google/go-containerregistry/releases/latest) version.

   ```sh
   $ VERSION=$(curl -s "https://api.github.com/repos/google/go-containerregistry/releases/latest" | jq -r '.tag_name')
   ```

   or set a specific version:

   ```sh
   $ VERSION=vX.Y.Z   # Version number with a leading v
   ```

1. Download the release.

   ```sh
   $ OS=Linux       # or Darwin, Windows
   $ ARCH=x86_64    # or arm64, x86_64, armv6, i386, s390x
   $ curl -sL "https://github.com/google/go-containerregistry/releases/download/${VERSION}/go-containerregistry_${OS}_${ARCH}.tar.gz" > go-containerregistry.tar.gz
   ```

1. Verify the signature. We generate [SLSA 3 provenance](https://slsa.dev) using
   the OpenSSF's [slsa-framework/slsa-github-generator](https://github.com/slsa-framework/slsa-github-generator).
   To verify our release, install the verification tool from [slsa-framework/slsa-verifier#installation](https://github.com/slsa-framework/slsa-verifier#installation)
   and verify as follows:

   ```sh
   $ curl -sL https://github.com/google/go-containerregistry/releases/download/${VERSION}/multiple.intoto.jsonl > provenance.intoto.jsonl
   $ # NOTE: You may be using a different architecture.
   $ slsa-verifier-linux-amd64 verify-artifact go-containerregistry.tar.gz --provenance-path provenance.intoto.jsonl --source-uri github.com/google/go-containerregistry --source-tag "${VERSION}"
     PASSED: Verified SLSA provenance
   ```

1. Unpack it in the PATH.

   ```sh
   $ tar -zxvf go-containerregistry.tar.gz -C /usr/local/bin/ crane
   ```

### Install manually

Install manually:

```sh
go install github.com/google/go-containerregistry/cmd/crane@latest
```

### Install via brew

If you're macOS user and using [Homebrew](https://brew.sh/), you can install via brew command:

```sh
$ brew install crane
```

### Install on Arch Linux

If you're an Arch Linux user you can install via pacman command:

```sh
$ pacman -S crane
```

### Setup on GitHub Actions

You can use the [`setup-crane`](https://github.com/imjasonh/setup-crane) action
to install `crane` and setup auth to [GitHub Container
Registry](https://github.com/features/packages) in a GitHub Action workflow:

```
steps:
- uses: imjasonh/setup-crane@v0.1
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
