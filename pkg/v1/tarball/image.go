// Copyright 2018 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tarball

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/internal/and"
	"github.com/google/go-containerregistry/pkg/v1/internal/gzip"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type image struct {
	opener        Opener
	tarmanifest   *Manifest
	config        []byte
	configName    v1.Hash
	imgDescriptor *Descriptor

	tag *name.Tag

	layerByDiffID map[v1.Hash]v1.Layer

	manifestLock sync.Mutex // Protects manifest
	manifest     *v1.Manifest
}

var _ v1.Image = (*image)(nil)

// Opener is a thunk for opening a tar file.
type Opener func() (io.ReadCloser, error)

func pathOpener(path string) Opener {
	return func() (io.ReadCloser, error) {
		return os.Open(path)
	}
}

// ImageFromPath returns a v1.Image from a tarball located on path.
func ImageFromPath(path string, tag *name.Tag) (v1.Image, error) {
	return Image(pathOpener(path), tag)
}

// Image exposes an image from the tarball at the provided path.
func Image(opener Opener, tag *name.Tag) (v1.Image, error) {
	img := &image{
		opener:        opener,
		tag:           tag,
		layerByDiffID: map[v1.Hash]v1.Layer{},
	}

	if err := img.loadTarDescriptorAndConfig(); err != nil {
		return nil, err
	}

	return img, nil
}

func (i *image) MediaType() (types.MediaType, error) {
	return types.DockerManifestSchema2, nil
}

// Descriptor stores the manifest data for a single image inside a `docker save` tarball.
type Descriptor struct {
	Config   string
	RepoTags []string
	Layers   []string

	// Tracks foreign layer info. Key is DiffID.
	LayerSources map[v1.Hash]v1.Descriptor `json:",omitempty"`
}

// Manifest represents the manifests of all images as the `manifest.json` file in a `docker save` tarball.
type Manifest []Descriptor

func (m Manifest) findDescriptor(tag *name.Tag) (*Descriptor, error) {
	if tag == nil {
		if len(m) != 1 {
			return nil, errors.New("tarball must contain only a single image to be used with tarball.Image")
		}
		return &(m)[0], nil
	}
	for _, img := range m {
		for _, tagStr := range img.RepoTags {
			repoTag, err := name.NewTag(tagStr)
			if err != nil {
				return nil, err
			}

			// Compare the resolved names, since there are several ways to specify the same tag.
			if repoTag.Name() == tag.Name() {
				return &img, nil
			}
		}
	}
	return nil, fmt.Errorf("tag %s not found in tarball", tag)
}

func (i *image) loadTarDescriptorAndConfig() error {
	m, _, err := extractFileFromTar(i.opener, "manifest.json")
	if err != nil {
		return err
	}
	defer m.Close()

	if err := json.NewDecoder(m).Decode(&i.tarmanifest); err != nil {
		return err
	}

	if i.tarmanifest == nil {
		return errors.New("no valid manifest.json in tarball")
	}

	i.imgDescriptor, err = i.tarmanifest.findDescriptor(i.tag)
	if err != nil {
		return err
	}

	cfg, _, err := extractFileFromTar(i.opener, i.imgDescriptor.Config)
	if err != nil {
		return err
	}
	defer cfg.Close()

	i.config, err = ioutil.ReadAll(cfg)
	if err != nil {
		return err
	}
	return nil
}

func (i *image) RawConfigFile() ([]byte, error) {
	return i.config, nil
}

type errNotfound struct {
	filePath string
}

func (e errNotfound) Error() string {
	return fmt.Sprintf("file %s not found in tar", e.filePath)
}

// TODO(#874): We could make this streamable with very careful sequencing during writes.
// The order that we'd need to read things:
// 1. manifest.json
// 2. i.imgDescriptor.Config
// 3. Layers(), which is the hard part...
//
// We need to encounter each layer in the tarball in the same order as we return
// them from Layers(). We also need to stop Peeking at the first 2 bytes eagerly.
// If we control what we're writing, we can populate as much info as we need in
// LayerSources, so it's _possible_, but every Write implementation would need
// to know to only write one layer at a time (or we could block).
func extractFileFromTar(opener Opener, filePath string) (io.ReadCloser, *tar.Header, error) {
	f, err := opener()
	if err != nil {
		return nil, nil, err
	}
	close := true
	defer func() {
		if close {
			f.Close()
		}
	}()

	tf := tar.NewReader(f)
	for {
		hdr, err := tf.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		if hdr.Name == filePath {
			close = false
			return &and.ReadCloser{
				Reader:    tf,
				CloseFunc: f.Close,
			}, hdr, nil
		}
	}
	return nil, nil, errNotfound{filePath}
}

func (i *image) ConfigFile() (*v1.ConfigFile, error) {
	return partial.ConfigFile(i)
}

func (i *image) ConfigName() (v1.Hash, error) {
	return i.configName, nil
}

func (i *image) Layers() ([]v1.Layer, error) {
	cf, err := i.ConfigFile()
	if err != nil {
		return nil, err
	}

	layers := []v1.Layer{}
	for idx, fp := range i.imgDescriptor.Layers {
		fp := fp
		diffid := cf.RootFS.DiffIDs[idx]

		l, err := i.makeLayer(fp, diffid)
		if err != nil {
			return nil, err
		}

		layers = append(layers, l)
		i.layerByDiffID[diffid] = l

	}
	return layers, nil
}

func (i *image) Digest() (v1.Hash, error) {
	return partial.Digest(i)
}

func (i *image) RawManifest() ([]byte, error) {
	return partial.RawManifest(i)
}

func (i *image) Size() (int64, error) {
	return partial.Size(i)
}

func (i *image) Manifest() (*v1.Manifest, error) {
	// This feels a lot like mutate, but we can't use mutate because mutate
	// stupidly depends on tarball (for now?).
	i.manifestLock.Lock()
	defer i.manifestLock.Unlock()
	if i.manifest != nil {
		return i.manifest, nil
	}

	b, err := i.RawConfigFile()
	if err != nil {
		return nil, err
	}

	cfgHash, cfgSize, err := v1.SHA256(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	i.configName = cfgHash

	i.manifest = &v1.Manifest{
		SchemaVersion: 2,
		MediaType:     types.DockerManifestSchema2,
		Config: v1.Descriptor{
			MediaType: types.DockerConfigJSON,
			Size:      cfgSize,
			Digest:    cfgHash,
		},
	}

	layers, err := i.Layers()
	if err != nil {
		return nil, err
	}

	for _, l := range layers {
		desc, err := partial.Descriptor(l)
		if err != nil {
			return nil, err
		}
		i.manifest.Layers = append(i.manifest.Layers, *desc)
	}
	return i.manifest, nil
}

func (i *image) LayerByDiffID(h v1.Hash) (v1.Layer, error) {
	if l, ok := i.layerByDiffID[h]; ok {
		return l, nil
	}
	return nil, fmt.Errorf("diff id %q not found", h)
}

func (i *image) LayerByDigest(h v1.Hash) (v1.Layer, error) {
	if h == i.configName {
		return partial.ConfigLayer(i)
	}
	diffid, err := partial.BlobToDiffID(i, h)
	if err != nil {
		return nil, err
	}
	return i.LayerByDiffID(diffid)
}

func (i *image) makeLayer(filename string, diffid v1.Hash) (v1.Layer, error) {
	layerOpener, err := i.makeLayerOpener(filename, diffid)
	if err != nil {
		if _, ok := err.(errNotfound); !ok {
			return nil, err
		}
		// If the layer isn't in the tarball, that's okay, we might be dealing
		// with a foreign layer. Just stub it out for now -- we have all the
		// metadata required to reconstruct the manifest in LayerSources.
		if desc, ok := i.imgDescriptor.LayerSources[diffid]; ok {
			return &sparseLayer{
				diffid: diffid,
				desc:   desc,
				err:    err,
			}, nil
		}
	}

	return LayerFromOpener(layerOpener)

}

// We're going to do a few cheeky things here:
// 1. Implement a cheap DiffID method so that we can avoid recomputing it.
// 2. Peek at the first few bytes of this layer to see if it's compressed or not.
//   2a. If compressed, implement cheap Size method.
//   2b. If uncompressed, implement cheap UncompressedSize method.
// 3. If we have LayerSources info, use that to implement Descriptor.
func (i *image) makeLayerOpener(filename string, diffid v1.Hash) (Opener, error) {
	rc, hdr, err := extractFileFromTar(i.opener, filename)
	if err != nil {
		return nil, err
	}

	compressed, buf, err := gzip.Is(rc)
	if err != nil {
		return nil, err
	}

	brc := &and.ReadCloser{
		Reader:    buf,
		CloseFunc: rc.Close,
	}

	// The first time this gets called, we reuse the buffer from gzip.Is.
	// Every subsequent call re-extracts the layer from the tarball.
	reopen := reopener(brc, i.makeFileOpener(filename))

	return func() (io.ReadCloser, error) {
		rc, err := reopen()
		if err != nil {
			return nil, err
		}

		if desc, foreign := i.imgDescriptor.LayerSources[diffid]; foreign {
			return &foreignOpener{
				ReadCloser: rc,
				sparseLayer: sparseLayer{
					desc:   desc,
					diffid: diffid,
				},
			}, nil
		}

		if compressed {
			return &compressedOpener{
				ReadCloser: rc,
				hdr:        hdr,
				diffid:     diffid,
			}, nil
		}

		return &uncompressedOpener{
			ReadCloser: rc,
			hdr:        hdr,
			diffid:     diffid,
		}, nil
	}, nil
}

func (i *image) makeFileOpener(filename string) Opener {
	return func() (io.ReadCloser, error) {
		rc, _, err := extractFileFromTar(i.opener, filename)
		return rc, err
	}
}

// compressedOpener returns gzipped bytes and cheaply knows its Size and DiffID.
type compressedOpener struct {
	io.ReadCloser
	hdr    *tar.Header
	diffid v1.Hash
}

func (cl *compressedOpener) DiffID() (v1.Hash, error) {
	return cl.diffid, nil
}

func (cl *compressedOpener) Size() (int64, error) {
	return cl.hdr.Size, nil
}

// uncompressedOpener returns uncompressed bytes and cheaply knows its UncompressedSize and DiffID.
type uncompressedOpener struct {
	io.ReadCloser
	hdr    *tar.Header
	diffid v1.Hash
}

func (ul *uncompressedOpener) DiffID() (v1.Hash, error) {
	return ul.diffid, nil
}

func (ul *uncompressedOpener) UncompressedSize() (int64, error) {
	return ul.hdr.Size, nil
}

// sparseLayer represents a layer which is not actually present in this tarball
// but for which we have enough metadata to implement most of v1.Layer. Anything
// that attempts to actually read the contents directly will get an error.
type sparseLayer struct {
	desc   v1.Descriptor
	diffid v1.Hash
	err    error
}

func (sl *sparseLayer) DiffID() (v1.Hash, error) {
	return sl.diffid, nil
}

func (sl *sparseLayer) Descriptor() (*v1.Descriptor, error) {
	return &sl.desc, nil
}

func (sl *sparseLayer) Compressed() (io.ReadCloser, error) {
	// TODO: We _could_ fetch things from URLs.
	return nil, sl.err
}

func (sl *sparseLayer) Uncompressed() (io.ReadCloser, error) {
	// TODO: We _could_ fetch things from URLs.
	return nil, sl.err
}

func (sl *sparseLayer) Digest() (v1.Hash, error) {
	return sl.desc.Digest, nil
}

func (sl *sparseLayer) MediaType() (types.MediaType, error) {
	return sl.desc.MediaType, nil
}

func (sl *sparseLayer) Size() (int64, error) {
	return sl.desc.Size, nil
}

// foreignOpener is like sparseLayer, but we actually have the bytes available.
// TODO: We could Peek at the bytes to determine if they're gzipped and expose
// UncompressedSize if not.
type foreignOpener struct {
	io.ReadCloser
	sparseLayer
}
