## crane annotate

Modify image or index annotations. The manifest is updated there on the registry.

```
crane annotate [flags]
```

### Options

```
  -a, --annotation stringToString   New annotations to add (default [])
  -h, --help                        help for annotate
  -t, --tag string                  New tag reference to apply to annotated image/index. If not provided, push by digest to the original repository.
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

