## crane ls

List the tags in a repo

```
crane ls REPO [flags]
```

### Options

```
      --full-ref           (Optional) if true, print the full image reference
  -h, --help               help for ls
  -O, --omit-digest-tags   (Optional), if true, omit digest tags (e.g., ':sha256-...')
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

