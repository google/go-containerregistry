# `crane` Recipes

Useful tips and things you can do with `crane` and other standard tools.

### Extract a single file from an image

```
crane export ubuntu - | tar -Oxf - etc/passwd
```

Note: Be sure to remove the leading `/` from the path (not `/etc/passwd`)

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
