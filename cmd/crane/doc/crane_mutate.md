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
  -e, --env keyToValue              New envvar to add
      --exposed-ports strings       New ports to expose
  -h, --help                        help for mutate
  -l, --label stringToString        New labels to add (default [])
  -o, --output string               Path to new tarball of resulting image
      --repo string                 Repository to push the mutated image to. If provided, push by digest to this repository.
      --set-platform string         New platform to set in the form os/arch[/variant][:osversion] (e.g. linux/amd64)
  -t, --tag string                  New tag reference to apply to mutated image. If not provided, push by digest to the original image repository.
  -u, --user string                 New user to set
  -w, --workdir string              New working dir to set
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

