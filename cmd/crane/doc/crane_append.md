## crane append

Append contents of a tarball to a remote image

### Synopsis

This sub-command pushes an image based on an (optional)
base image, with appended layers containing the contents of the
provided tarballs.

If the base image is a Windows base image (i.e., its config.OS is "windows"),
the contents of the tarballs will be modified to be suitable for a Windows
container image.

```
crane append [flags]
```

### Options

```
  -b, --base string                  Name of base image to append to
  -h, --help                         help for append
  -f, --new_layer strings            Path to tarball to append to image
  -t, --new_tag string               Tag to apply to resulting image
      --oci-empty-base               If true, empty base image will have OCI media types instead of Docker
  -o, --output string                Path to new tarball of resulting image
      --set-base-image-annotations   If true, annotate the resulting image as being based on the base image
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

