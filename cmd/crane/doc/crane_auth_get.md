## crane auth get

Implements a credential helper

```
crane auth get [REGISTRY_ADDR] [flags]
```

### Examples

```
  # Read configured credentials for reg.example.com
  $ echo "reg.example.com" | crane auth get
  {"username":"AzureDiamond","password":"hunter2"}
  # or
  $ crane auth get reg.example.com
  {"username":"AzureDiamond","password":"hunter2"}
```

### Options

```
  -h, --help   help for get
```

### Options inherited from parent commands

```
      --allow-nondistributable-artifacts   Allow pushing non-distributable (foreign) layers
      --insecure                           Allow image references to be fetched without TLS
      --platform platform                  Specifies the platform in the form os/arch[/variant][:osversion] (e.g. linux/amd64). (default all)
  -v, --verbose                            Enable debug logs
```

### SEE ALSO

* [crane auth](crane_auth.md)	 - Log in or access credentials

