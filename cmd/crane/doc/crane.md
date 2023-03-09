## crane

Crane is a tool for managing container images

```
crane [flags]
```

### Options

```
      --allow-nondistributable-artifacts   Allow pushing non-distributable (foreign) layers
  -h, --help                               help for crane
      --insecure                           Allow image references to be fetched without TLS
      --platform platform                  Specifies the platform in the form os/arch[/variant][:osversion] (e.g. linux/amd64). (default all)
  -v, --verbose                            Enable debug logs
```

### SEE ALSO

* [crane append](crane_append.md)	 - Append contents of a tarball to a remote image
* [crane auth](crane_auth.md)	 - Log in or access credentials
* [crane blob](crane_blob.md)	 - Read a blob from the registry
* [crane catalog](crane_catalog.md)	 - List the repos in a registry
* [crane config](crane_config.md)	 - Get the config of an image
* [crane copy](crane_copy.md)	 - Efficiently copy a remote image from src to dst while retaining the digest value
* [crane delete](crane_delete.md)	 - Delete an image reference from its registry
* [crane digest](crane_digest.md)	 - Get the digest of an image
* [crane export](crane_export.md)	 - Export filesystem of a container image as a tarball
* [crane flatten](crane_flatten.md)	 - Flatten an image's layers into a single layer
* [crane index](crane_index.md)	 - Modify an image index.
* [crane ls](crane_ls.md)	 - List the tags in a repo
* [crane manifest](crane_manifest.md)	 - Get the manifest of an image
* [crane mutate](crane_mutate.md)	 - Modify image labels and annotations. The container must be pushed to a registry, and the manifest is updated there.
* [crane pull](crane_pull.md)	 - Pull remote images by reference and store their contents locally
* [crane push](crane_push.md)	 - Push local image contents to a remote registry
* [crane rebase](crane_rebase.md)	 - Rebase an image onto a new base image
* [crane registry](crane_registry.md)	 - 
* [crane tag](crane_tag.md)	 - Efficiently tag a remote image
* [crane validate](crane_validate.md)	 - Validate that an image is well-formed
* [crane version](crane_version.md)	 - Print the version

