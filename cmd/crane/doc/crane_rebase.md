## crane rebase

Rebase an image onto a new base image

```
crane rebase [flags]
```

### Options

```
  -h, --help              help for rebase
      --new_base string   New base image to insert
      --old_base string   Old base image to remove
      --original string   Original image to rebase (DEPRECATED: use positional arg instead)
      --rebased string    Tag to apply to rebased image (DEPRECATED: use --tag)
  -t, --tag string        Tag to apply to rebased image
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

