# `layout`

[![GoDoc](https://godoc.org/github.com/google/go-containerregistry/pkg/v1/layout?status.svg)](https://godoc.org/github.com/google/go-containerregistry/pkg/v1/layout)

The `layout` package implements support for interacting with an [OCI Image Layout](https://github.com/opencontainers/image-spec/blob/master/image-layout.md).

## Reading a trusted local layout with symlinked blobs

The existing `Path.Blob`, `Path.Bytes`, `Path.Image`, `Path.ImageIndex`, and
`ImageIndexFromPath` functions reject final symlink entries in the blobs
directory. Their signatures and default behavior are unchanged.

Some local build systems and shared blob caches intentionally use symlinked
blobs. A caller that trusts the layout producer can opt in for a particular
reader:

```go
func loadTrustedIndex(dir string) (v1.ImageIndex, error) {
    path, err := layout.FromPath(dir)
    if err != nil {
        return nil, err
    }
    return path.Reader(layout.WithAllowFinalSymlinks()).ImageIndex()
}
```

The returned index retains the reader policy for nested indexes, image
manifests, configuration blobs, and image layers.

`path.Reader()` uses the same strict policy as `Path`. Creating an opted-in
reader does not modify `path`, another reader, or any process-wide setting.
The option applies to reads and to `Reader.GarbageCollect`, which traverses the
layout using that reader's policy. Existing `Path` methods, including
`Path.GarbageCollect`, remain strict by default, and write operations are
unchanged.

Enable this option only for caller-trusted local layout producers. A final
symlink can redirect a blob read to another file accessible to the reader, so
the meaningful trust boundary is an untrusted layout producer versus a more
privileged reader or uploader. Trusted build-generated layouts may legitimately
use symlinks to share layers, but the opt-in should remain scoped to the reader
handling that layout. Targets may be outside the layout directory, and parent
path components retain normal operating-system resolution; neither mode is a
complete directory sandbox. In the opted-in mode, resolved targets must still
be regular files, and the file identity observed before opening must match the
opened file.

The check happens when a blob is opened; constructing an image or index can
leave some reads deferred. For example, an uploader may skip opening layer
blobs that already exist at its destination. The strict default is not an eager
scan of every entry in a layout. Neither policy freezes file contents against
concurrent mutation.
