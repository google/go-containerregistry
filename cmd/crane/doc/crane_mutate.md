## crane mutate

Modify image labels and annotations. The container must be pushed to a registry, and the manifest is updated there.

```
crane mutate [flags]
```

### Options

```
  -a, --annotation stringToString   New annotations to add (default [])
      --append strings              Path to tarball to append to image
      --cmd strings                 New cmd to set
      --entrypoint strings          New entrypoint to set
  -e, --env stringToString          New envvar to add (default [])
  -h, --help                        help for mutate
  -l, --label stringToString        New labels to add (default [])
  -t, --tag string                  New tag to apply to mutated image. If not provided, push by digest to the original image repository.
```

### Options inherited from parent commands

```
      --insecure            Allow image references to be fetched without TLS
      --platform platform   Specifies the platform in the form os/arch[/variant][:osversion] (e.g. linux/amd64). (default all)
  -v, --verbose             Enable debug logs
```

### SEE ALSO

* [crane](crane.md)	 - Crane is a tool for managing container images

