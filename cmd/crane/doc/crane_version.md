## crane version

Print the version

### Synopsis

The version string is completely dependent on how the binary was built, so you should not depend on the version format. It may change without notice.

This could be an arbitrary string, if specified via -ldflags.
This could also be the go module version, if built with go modules (often "(devel)").

```
crane version [flags]
```

### Options

```
  -h, --help   help for version
```

### Options inherited from parent commands

```
  -v, --verbose   Enable debug logs
```

### SEE ALSO

* [crane](crane.md)	 - Crane is a tool for managing container images

