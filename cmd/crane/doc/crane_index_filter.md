## crane index filter

Modifies a remote index by filtering based on platform.

```
crane index filter [flags]
```

### Examples

```
  # Filter out weird platforms from ubuntu, copy result to example.com/ubuntu
  crane index filter ubuntu --platform linux/amd64 --platform linux/arm64 -t example.com/ubuntu

  # Filter out any non-linux platforms, push to example.com/hello-world
  crane index filter hello-world --platform linux -t example.com/hello-world

  # Same as above, but in-place
  crane index filter example.com/hello-world:some-tag --platform linux
```

### Options

```
  -h, --help                   help for filter
      --platform platform(s)   Specifies the platform(s) to keep from base in the form os/arch[/variant][:osversion][,<platform>] (e.g. linux/amd64).
  -t, --tag string             Tag to apply to resulting image
```

### Options inherited from parent commands

```
      --allow-nondistributable-artifacts   Allow pushing non-distributable (foreign) layers
      --insecure                           Allow image references to be fetched without TLS
  -v, --verbose                            Enable debug logs
```

### SEE ALSO

* [crane index](crane_index.md)	 - Modify an image index.

