## crane auth get

Implements a credential helper

### Synopsis

Implements a credential helper

```
crane auth get [flags]
```

### Examples

```
  # Read configured credentials for reg.example.com
  echo "reg.example.com" | crane auth get
  {"username":"AzureDiamond","password":"hunter2"}
```

### Options

```
  -h, --help   help for get
```

### Options inherited from parent commands

```
      --insecure            Allow image references to be fetched without TLS
      --platform platform   Specifies the platform in the form os/arch[/variant] (e.g. linux/amd64). (default all)
  -v, --verbose             Enable debug logs
```

### SEE ALSO

* [crane auth](crane_auth.md)	 - Log in or access credentials

