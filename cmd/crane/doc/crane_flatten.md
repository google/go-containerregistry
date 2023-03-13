## crane flatten

Flatten an image's layers into a single layer

Here's an example. If an image had three layers:

layer 1 (bottom): file at /tmp/foo containing the text "foo"
layer 2 (middle): file at /tmp/bar containing the text "bar"
layer 3 (top): file at /tmp/foo containing the text "blah"
Flattening this image would produce an image with one layer:

layer: file at /tmp/foo containing the text "blah", file at /tmp/bar containing the text "bar"
In this example it reduced the size of the image -- the version of the file with contents "foo" is gone from the image because it was shadowed by the top-layer file with the same name.

This can be a drawback in some cases, due to caching. If there's a base layer that's shared by many images, you don't need to pull the contents of all three files to pull the example image -- the bottom two layers might already be present, so you'd only pull "blah". Common base images like ubuntu, debian, alpine, etc. are very often shared by many images, and their layers are heavily cached and don't usually have to be pulled to pull a new image based on those images.

```
crane flatten [flags]
```

### Options

```
  -h, --help         help for flatten
  -t, --tag string   New tag to apply to flattened image. If not provided, push by digest to the original image repository.
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

