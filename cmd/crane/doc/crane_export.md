## crane export

Export filesystem of a container image as a tarball

```
crane export IMAGE|- TARBALL|- [flags]
```

### Examples

```
  # Write tarball to stdout
  crane export ubuntu -

  # Write tarball to file
  crane export ubuntu ubuntu.tar

  # Read image from stdin
  crane export - ubuntu.tar
```

### Options

```
  -h, --help   help for export
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

