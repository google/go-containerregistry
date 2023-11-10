## crane tag

Efficiently tag a remote image

### Synopsis

Tag remote image without downloading it.

This differs slightly from the "copy" command in a couple subtle ways:

1. You don't have to specify the entire repository for the tag you're adding. For example, these two commands are functionally equivalent:
```
crane cp registry.example.com/library/ubuntu:v0 registry.example.com/library/ubuntu:v1
crane tag registry.example.com/library/ubuntu:v0 v1
```

2. We can skip layer existence checks because we know the manifest already exists. This makes "tag" slightly faster than "copy".

```
crane tag IMG TAG [flags]
```

### Examples

```
# Add a v1 tag to ubuntu
crane tag ubuntu v1
```

### Options

```
  -h, --help   help for tag
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

