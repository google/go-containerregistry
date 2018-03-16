# Test Data

The files in this directory are test data for tarball construction.

## test_image.tar

This is a valid, single-layer tarball. The contents are:

```shell
$ tar -tf test_image.tar
47b20a5c857d1adc47d5ee911b1e73f8fdd17e81459f7d241a918ff797d3ef9e.json
c47412cb682fcf4ae43f536a788069f78552f928a55eeb09ba1455e0f9a2306e/VERSION
c47412cb682fcf4ae43f536a788069f78552f928a55eeb09ba1455e0f9a2306e/layer.tar
c47412cb682fcf4ae43f536a788069f78552f928a55eeb09ba1455e0f9a2306e/json
repositories
manifest.json
```

## no_manifest.tar

This is the same tarball as above, but with no manifest.json.

```shell
$ tar -tf no_manifest.tar
47b20a5c857d1adc47d5ee911b1e73f8fdd17e81459f7d241a918ff797d3ef9e.json
c47412cb682fcf4ae43f536a788069f78552f928a55eeb09ba1455e0f9a2306e/VERSION
c47412cb682fcf4ae43f536a788069f78552f928a55eeb09ba1455e0f9a2306e/layer.tar
c47412cb682fcf4ae43f536a788069f78552f928a55eeb09ba1455e0f9a2306e/json
repositories
```

## bundle.tar

This tarball contains two images.

```shell
$ tar -tf v1/tarball/testdata/bundle.tar
2cc9eac29a6b9cf786b31bcbb6ec7f4c4cb3f32e2a35f3be103d52a4f18556af.json
6c8916f083be5e43bbab5b877cd3179cc419784495fb0d8321243c3c35996dfe.json
783fd37019ed70c8eaeaf4593164de8582afa9109a688b642e9ed2658fb98e6a/VERSION
783fd37019ed70c8eaeaf4593164de8582afa9109a688b642e9ed2658fb98e6a/layer.tar
783fd37019ed70c8eaeaf4593164de8582afa9109a688b642e9ed2658fb98e6a/json
f0a8356e32aa52352b06de343c1e45924256f30e76715808d9baa8539bb07ea6/VERSION
f0a8356e32aa52352b06de343c1e45924256f30e76715808d9baa8539bb07ea6/layer.tar
f0a8356e32aa52352b06de343c1e45924256f30e76715808d9baa8539bb07ea6/json
repositories
manifest.json
```
