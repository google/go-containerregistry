## crane validate

Validate that an image is well-formed

```
crane validate [flags]
```

### Options

```
      --fast             Skip downloading/digesting layers
  -h, --help             help for validate
      --remote string    Name of remote image to validate
      --tarball string   Path to tarball to validate
```

### Options inherited from parent commands

```
      --dial-timeout duration   Modify the dial timeout used to contact the registry. (default 5s)
      --insecure                Allow image references to be fetched without TLS
      --osversion string        Specifies the OS version.
      --platform platform       Specifies the platform in the form os/arch[/variant] (e.g. linux/amd64). (default all)
  -v, --verbose                 Enable debug logs
```

### SEE ALSO

* [crane](crane.md)	 - Crane is a tool for managing container images

