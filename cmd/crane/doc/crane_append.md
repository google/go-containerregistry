## crane append

Append contents of a tarball to a remote image

### Synopsis

Append contents of a tarball to a remote image

```
crane append [flags]
```

### Options

```
  -b, --base string         Name of base image to append to
  -h, --help                help for append
  -f, --new_layer strings   Path to tarball to append to image
  -t, --new_tag string      Tag to apply to resulting image
  -o, --output string       Path to new tarball of resulting image
```

### Options inherited from parent commands

```
      --insecure            Allow image references to be fetched without TLS
      --platform platform   Specifies the platform in the form os/arch[/variant] (e.g. linux/amd64). (default all)
  -v, --verbose             Enable debug logs
```

### SEE ALSO

* [crane](crane.md)	 - Crane is a tool for managing container images

