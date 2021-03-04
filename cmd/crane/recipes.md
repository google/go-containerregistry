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

### Diff two configs

```
diff <(crane config busybox:1.32 | jq) <(crane config busybox:1.33 | jq)
```

### Diff two manifests

```
diff <(crane manifest busybox:1.32 | jq) <(crane manifest busybox:1.33 | jq)
```
