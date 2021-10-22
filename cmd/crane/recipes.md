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
