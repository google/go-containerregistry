## crane mutate

Modify image labels and annotations

### Synopsis

Modify image labels and annotations

```
crane mutate [flags]
```

### Options

```
      --entrypoint string   New entrypoing to set
  -h, --help                help for mutate
  -l, --label strings       New labels to add
  -t, --tag string          New tag to apply to mutated image. If not provided, push by digest to the original image repository.
```

### Options inherited from parent commands

```
      --insecure            Allow image references to be fetched without TLS
      --platform platform   Specifies the platform in the form os/arch[/variant] (e.g. linux/amd64). (default all)
  -v, --verbose             Enable debug logs
```

### SEE ALSO

* [crane](crane.md)	 - Crane is a tool for managing container images

