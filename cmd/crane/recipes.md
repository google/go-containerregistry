# `crane` Recipes

Useful tips and things you can do with `crane` and other standard tools.

### List files in an image

```
crane export ubuntu - | tar -tvf - | less
```

### Extract a single file from an image

```
crane export ubuntu - | tar -Oxf - etc/passwd
```

Note: Be sure to remove the leading `/` from the path (not `/etc/passwd`). This behavior will not follow symlinks.

### Bundle directory contents into an image

```
crane append -f <(tar -f - -c some-dir/) -t ${IMAGE}
```

By default, this produces an image with one layer containing the directory contents. Add `-b ${BASE_IMAGE}` to append the layer to a base image instead.

You can extend this even further with `crane mutate`, to make an executable in the appended layer the image's entrypoint.

```
crane mutate ${IMAGE} --entrypoint=some-dir/entrypoint.sh
```

Because `crane append` emits the full image reference, these calls can even be chained together:

```
crane mutate $(
  crane append -f <(tar -f - -c some-dir/) -t ${IMAGE}
) --entrypoint=some-dir/entrypoint.sh
```

This will bundle `some-dir/` into an image, push it, mutate its entrypoint to `some-dir/entrypoint.sh`, and push that new image by digest.

### Diff two configs

```
diff <(crane config busybox:1.32 | jq) <(crane config busybox:1.33 | jq)
```

### Diff two manifests

```
diff <(crane manifest busybox:1.32 | jq) <(crane manifest busybox:1.33 | jq)
```

### Diff filesystem contents

```
diff \
    <(crane export gcr.io/kaniko-project/executor:v1.6.0-debug - | tar -tvf - | sort) \
    <(crane export gcr.io/kaniko-project/executor:v1.7.0-debug - | tar -tvf - | sort)
```

This will show file size diffs and (unfortunately) modified time diffs.

With some work, you can use `cut` and other built-in Unix tools to ignore these diffs.

### Get total image size

Given an image manifest, you can calculate the total size of all layer blobs and the image's config blob using `jq`:

```
crane manifest gcr.io/buildpacks/builder:v1 | jq '.config.size + ([.layers[].size] | add)'
```

This will produce a number of bytes, which you can make human-readable by passing to [`numfmt`](https://www.gnu.org/software/coreutils/manual/html_node/numfmt-invocation.html)

```
crane manifest gcr.io/buildpacks/builder:v1 | jq '.config.size + ([.layers[].size] | add)' | numfmt --to=iec
```

For image indexes, you can pass the `--platform` flag to `crane` to get a platform-specific image.

### Filter irrelevant platforms from a multi-platform image

Perhaps you use a base image that supports a wide variety of exotic platforms, but you only care about linux/amd64 and linux/arm64.
If you want to copy that base image into a different registry, you will end up with a bunch of images you don't use.
You can filter the base to include only platforms that are relevant to you.

```
crane index filter ubuntu --platform linux/amd64 --platform linux/arm64 -t ${IMAGE}
```

Note that this will obviously modify the digest of the multi-platform image you're using, so this may invalidate other artifacts that reference it, e.g. signatures.

### Create a multi-platform image from scratch

If you have a bunch of platform-specific images that you want to turn into a multi-platform image, `crane index append` can do that:

```
crane index append -t ${IMAGE} \
  -m ubuntu@sha256:c985bc3f77946b8e92c9a3648c6f31751a7dd972e06604785e47303f4ad47c4c \
  -m ubuntu@sha256:61bd0b97000996232eb07b8d0e9375d14197f78aa850c2506417ef995a7199a7
```

Note that this is less flexible than [`manifest-tool`](https://github.com/estesp/manifest-tool) because it derives the platform from each image's config file, but it should work in most cases.
