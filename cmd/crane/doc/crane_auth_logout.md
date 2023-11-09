## crane auth logout

Log out of a registry

```
crane auth logout [SERVER] [flags]
```

### Examples

```
  # Log out of reg.example.com
  crane auth logout reg.example.com
```

### Options

```
  -h, --help   help for logout
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

