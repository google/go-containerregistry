## crane index append

Append manifests to a remote index.

### Synopsis

This sub-command pushes an index based on an (optional) base index, with appended manifests.

The platform for appended manifests is inferred from the config file or omitted if that is infeasible.

```
crane index append [flags]
```

### Examples

```
 # Append a windows hello-world image to ubuntu, push to example.com/hello-world:weird
  crane index append ubuntu -m hello-world@sha256:87b9ca29151260634b95efb84d43b05335dc3ed36cc132e2b920dd1955342d20 -t example.com/hello-world:weird

  # Create an index from scratch for etcd.
  crane index append -m registry.k8s.io/etcd-amd64:3.4.9 -m registry.k8s.io/etcd-arm64:3.4.9 -t example.com/etcd
```

### Options

```
      --docker-empty-base   If true, empty base index will have Docker media types instead of OCI
      --flatten             If true, appending an index will append each of its children rather than the index itself (default true)
  -h, --help                help for append
  -m, --manifest strings    References to manifests to append to the base index
  -t, --tag string          Tag to apply to resulting image
```

### Options inherited from parent commands

```
      --allow-nondistributable-artifacts   Allow pushing non-distributable (foreign) layers
      --insecure                           Allow image references to be fetched without TLS
      --platform platform                  Specifies the platform in the form os/arch[/variant][:osversion] (e.g. linux/amd64). (default all)
  -v, --verbose                            Enable debug logs
```

### SEE ALSO

* [crane index](crane_index.md)	 - Modify an image index.

