## crane flatten

Flatten an image's layers into a single layer

```
crane flatten [flags]
```

### Options

```
  -h, --help         help for flatten
  -t, --tag string   New tag to apply to flattened image. If not provided, push by digest to the original image repository.
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

