## crane catalog

List the repos in a registry

```
crane catalog [REGISTRY] [flags]
```

### Examples

```
  # list the repos for reg.example.com
  $ crane catalog reg.example.com
```

### Options

```
  -h, --help   help for catalog
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

