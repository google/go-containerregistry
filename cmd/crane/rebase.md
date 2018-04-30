This command rewrites an image's manifest to replace layers in a base image
with layers in another version of that base image. It does so entirely with API
calls to the registry, so it doesn't have to download or upload full layer
blobs, if it can be avoided.

This can be useful if you want to produce container images that pick up
security or bug fixes in base images, without having to completely rebuild the
image from source. For instance, you might not have access to the original
source anymore, or you want to produce updated images without performing a full
rebuild.

*WARNING:* The image that results from such a rebase might not be valid in all
cases. More details below, but caveat emptor.

## Using `crane rebase`

For purposes of illustration, imagine you've built a container image
`gcr.io/my-project/my-app:latest`, containing your app, and based on some OS
image, for instance, `ubuntu`. A vulnerability has been found in the base
image, and a new fixed version has been released.

You could build your app image again, and its `FROM ubuntu` directive would
pick up the new base image release, but that requires a full rebuild of your
entire app from source, which might pull in other changes in dependencies. You
just want to release this critical bug fix, as quickly as possible.

Instead, you could use `crane rebase` to replace the vulnerable base image
layers in your image with new patched base image layers from the newly released
base image, without needing to rebuild from source, or indeed have access to
the source at all.

```
$ crane rebase \
  --original=gcr.io/my-project/my-app:latest \
  --old_base=ubuntu@sha256:deadbeef... \
  --new_base=ubuntu:latest \
  --rebased=gcr.io/my-project/my-app:rebased
```

This command would fetch the manifest for `original`, `old_base` and `new_base`,
check that `old_base` is indeed the basis for `original`, remove `old_base`'s
layers from `original` and replace them with `new_base`'s layers, then compute
and upload a new valid manifest for the image, tagged as `rebased`.

If the image is in Google Container Registry, you can determine `old_base` image
digests using `gcloud alpha container images describe <image>
--show-image-basis`.

## Rebase visualized

![rebase visualization](./rebase.png)

## Caveats

Rebasing has no visibility into what the container image contains, or what
constitutes a "valid" image. As a result, it's perfectly capable of producing an
image that's entirely invalid garbage. Rebasing arbitrary layers in an image is
not a good idea.

To help prevent garbage images, rebasing should only be done at a point in the
layer stack between "base" layers and "app" layers. These should adhere to some
contract about what "base" layers can be expected to produce, and what "app"
layers should expect from base layers.

In the example above, for instance, we assume that the "Ubuntu" base image is
adhering to some contract with downstream app layers, that it won't remove or
drastically change what it provides to the app layer. If the `new_base` layers
removed some installed package, or made a breaking change to the version of some
compiler expected by the uppermost app layers, the resulting rebased image might
be invalid.

In general, it's a good practice to tag rebased images to some other tag than
the `original` tag, perform some sanity checks, then tag the image to the
`original` tag once it's determined the image is valid.

There is ongoing work to standardize and advertise base image contract
adherence to make rebasing safer.

## Automatic Rebase Seam Detection

If an app image adheres to a strong contract with its base layers, the tool
that builds the app image can insert a hint into the image, in the form of a
`LABEL`, to help `crane rebase` detect the values of `--old_base` and
`--new_base` automatically.

The form of the `LABEL` is:

```
LABEL rebase <current-base-image-by-digest> <current-base-image-by-tag>
```

When `crane rebase` is asked to rebase an image without being passed
`--old_base` and `--new_base` explicitly, it will look for this label and fill
in `--old_base=<base-by-digest>` and `--new_base=<base-by-tag>`.

In this way, new releases of `<base-by-tag>` will automatically be considered
as `--new_base` for the app image, if the flags aren't passed explicitly.

The `crane rebase` tool injects this `LABEL` into the `--rebased` image it
produces, if `--new_base` is passed as a tag, to aid future rebase operations
on that image.

Using the example above, `gcr.io/my-project/my-app:rebased` contains the following label:

```
LABEL rebase ubuntu@sha256:facadecafe... ubuntu:latest
```

This supplies a hint to `crane rebase` that if `ubuntu16_04:latest` is updated,
it should be used as the new base for `:rebased` image, instead of
`ubuntu16_04@sha256:facadecafe...`, which is its current base.

Future rebase operations can be specified with just two flags:

```
$ crane rebase \
  --original=gcr.io/my-project/my-app:rebased \
  --rebased=gcr.io/my-project/my-app:rebased-again
```

### This code is experimental and might break you if not used correctly.
