# `ko`

`ko` is an **experimental** CLI to support rapid development of Go containers
with Kubernetes and minimal configuration.

## Installation

`ko` can be installed via:

```shell
go install github.com/google/go-containerregistry/cmd/ko
```

## The `ko` Model

`ko` is built around a very simple extension to Go's model for expressing
dependencies using [import paths](https://golang.org/doc/code.html#ImportPaths).

In Go, dependencies are expressed via blocks like:

```go
import (
    "github.com/google/go-containerregistry/authn"
    "github.com/google/go-containerregistry/name"
)
```

Similarly (as you can see above), Go binaries can be referenced via import
paths like `github.com/google/go-containerregistry/cmd/ko`.

**One of the goals of `ko` is to make containers invisible infrastructure.**
Simply replace image references in your Kubernetes yaml with the import path for
your Go binary, and `ko` will handle containerizing and publishing that
container image as needed.

For example, you might use the following in a Kubernetes `Deployment` resource:

```yaml
apiVersion: apps/v1beta1
kind: Deployment
metadata:
  name: hello-world
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: hello-world
        # This is the import path for the Go binary to build and run.
        image: github.com/mattmoor/examples/http/cmd/helloworld
        ports:
        - containerPort: 8080
```

**This extension of the Go convention enables us to create a tool with an
extremely fast development cycle and effectively zero configuration.**

## Usage

`ko` also three commands that interact with Kubernetes resources.

### `ko resolve`

`ko resolve` builds on the [model](#the-ko-model) above to determine the set of
Go import paths to build, containerize, and publish. It then outputs a
multi-document yaml of the resources to `STDOUT`, replacing import paths with
published image digests.

To determine where to publish the images, `ko` currently requires the
environment variable `KO_DOCKER_REPO` to be set to an acceptable docker
repository (e.g. `gcr.io/your-project`). This will likely change in a
future version.

Following the example above, this might result in:

```shell
export PROJECT_ID=$(gcloud config get-value core/project)
export KO_DOCKER_REPO="gcr.io/${PROJECT_ID}"
ko resolve -f deployment.yaml
apiVersion: apps/v1beta1
kind: Deployment
metadata:
  name: hello-world
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: hello-world
        # This is the digest of the published image containing the go binary
        # at the embedded import path.
        image: gcr.io/your-project/github.com/mattmoor/examples/http/cmd/helloworld@sha256:deadbeef
        ports:
        - containerPort: 8080
```

*Note that DockerHub does not currently support these multi-level names. We may
employ alternate naming strategies in the future to broaden support, but this
would sacrifice some amount of identifiability.*

### `ko apply`

`ko apply` is intended to parallel `kubectl apply`, but acts on the same
resolved output as `ko resolve` emits. It is expected that `ko apply` will act
as the vehicle for rapid iteration during development.  As changes are made to a
particular application, you can run: `ko apply -f unit.yaml` to rapidly
rebuild, repush, and redeploy their changes.

`ko apply` will invoke `kubectl apply` under the covers, and therefore apply
to whatever `kubectl` context is active.

### `ko delete`

`ko delete` simply passes through to `kubectl delete`, as with the `go`
commands. It is exposed purely out of convenience for cleaning up resources
created through `ko apply`.

## Relevance to Release Management

`ko` is also useful for helping manage releases. For example, if your project
periodically releases a set of images and configuration to launch those images
on a Kubernetes cluster, release binaries may be published and the configuration
generated via:

```shell
export PROJECT_ID=<YOUR RELEASE PROJECT>
export KO_DOCKER_REPO="gcr.io/${PROJECT_ID}"
ko resolve -f config/ > release.yaml
```

This will publish all of the binary components as container images to
`gcr.io/my-releases/...` and create a `release.yaml` file containing all of the
configuration for your application with inlined image references.

This resulting configuration may then be installed onto Kubernetes clusters via:

```shell
kubectl apply -f release.yaml
```


## Acknowledgements

This work is based heavily on learnings from having built the
[Docker](github.com/bazelbuild/rules_docker) and
[Kubernetes](github.com/bazelbuild/rules_k8s) support for
[Bazel](https://bazel.build). That work was presented
[here](https://www.youtube.com/watch?v=RS1aiQqgUTA).
