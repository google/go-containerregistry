## crane blob

Read a blob from the registry

```
crane blob BLOB [flags]
```

### Examples

```
crane blob ubuntu@sha256:4c1d20cdee96111c8acf1858b62655a37ce81ae48648993542b7ac363ac5c0e5 > blob.tar.gz
```

### Options

```
  -h, --help   help for blob
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

