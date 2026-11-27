## crane push

Push local image contents to a remote registry

### Synopsis

If the PATH is a directory, it will be read as an OCI image layout. Otherwise, PATH is assumed to be a docker-style tarball.

By default, final symlink entries in local OCI layout blobs are rejected. Use
--allow-layout-symlinks only for a layout from a trusted local producer. This
option affects local layout reads only and is independent of --insecure.

```
crane push PATH IMAGE [flags]
```

### Options

```
      --allow-layout-symlinks   allow final symlink entries in trusted local OCI layout blobs
  -h, --help                    help for push
      --image-refs string       path to file where a list of the published image references will be written
      --index                   push a collection of images as a single index, currently required if PATH contains multiple images
```

### Options inherited from parent commands

```
      --allow-nondistributable-artifacts   Allow pushing non-distributable (foreign) layers
      --insecure                           Allow image references to be fetched without TLS
      --platform platform                  Specifies the platform in the form os/arch[/variant][:osversion] (e.g. linux/amd64). (default all)
  -v, --verbose                            Enable debug logs
```

### SEE ALSO

* [crane](crane.md)	 - Crane is a tool for managing container images

