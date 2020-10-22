## crane export

Export contents of a remote image as a tarball

### Synopsis

Export contents of a remote image as a tarball

```
crane export IMAGE TARBALL [flags]
```

### Examples

```
  # Write tarball to stdout
  crane export ubuntu -

  # Write tarball to file
  crane export ubuntu ubuntu.tar
```

### Options

```
  -h, --help   help for export
```

### Options inherited from parent commands

```
      --insecure            Allow image references to be fetched without TLS
      --platform platform   Specifies the platform in the form os/arch[/variant] (e.g. linux/amd64). (default all)
  -v, --verbose             Enable debug logs
```

### SEE ALSO

* [crane](crane.md)	 - Crane is a tool for managing container images

