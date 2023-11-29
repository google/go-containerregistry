## crane registry serve

Serve a registry implementation

### Synopsis

This sub-command serves a registry implementation on an automatically chosen port (:0), $PORT or --address

The command blocks while the server accepts pushes and pulls.

Contents are can be stored in memory (when the process exits, pushed data is lost.), and disk (--disk).

```
crane registry serve [flags]
```

### Options

```
      --address string   Address to listen on
      --disk string      Path to a directory where blobs will be stored
  -h, --help             help for serve
```

### Options inherited from parent commands

```
      --allow-nondistributable-artifacts   Allow pushing non-distributable (foreign) layers
      --insecure                           Allow image references to be fetched without TLS
      --platform platform                  Specifies the platform in the form os/arch[/variant][:osversion] (e.g. linux/amd64). (default all)
  -v, --verbose                            Enable debug logs
```

### SEE ALSO

* [crane registry](crane_registry.md)	 - 

