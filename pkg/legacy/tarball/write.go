package tarball

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/google/go-containerregistry/pkg/legacy"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/pkg/errors"
)

// singleImageTarDescriptor is the struct used to represent a single image inside a `docker save` tarball.
type singleImageTarDescriptor struct {
	Config   string
	RepoTags []string
	Layers   []string

	// Tracks foreign layer info. Key is DiffID.
	LayerSources map[v1.Hash]v1.Descriptor `json:",omitempty"`
}

// tarDescriptor is the struct used inside the `manifest.json` file of a `docker save` tarball.
type tarDescriptor []singleImageTarDescriptor

// repositoriesTarDescriptor represents the repositories file inside a `docker save` tarball.
type repositoriesTarDescriptor map[string]map[string]string

// WriteToFile writes in the compressed format to a tarball, on disk.
// This is just syntactic sugar wrapping tarball.Write with a new file.
func WriteToFile(p string, ref name.Reference, img v1.Image) error {
	w, err := os.Create(p)
	if err != nil {
		return err
	}
	defer w.Close()

	return Write(ref, img, w)
}

// MultiWriteToFile writes in the V1 image tarball format to a tarball, on disk.
// This is just syntactic sugar wrapping tarball.MultiWrite with a new file.
func MultiWriteToFile(p string, tagToImage map[name.Tag]v1.Image) error {
	refToImage := make(map[name.Reference]v1.Image, len(tagToImage))
	for i, d := range tagToImage {
		refToImage[i] = d
	}
	return MultiRefWriteToFile(p, refToImage)
}

// MultiRefWriteToFile writes in the V1 image tarball format to a tarball, on disk.
// This is just syntactic sugar wrapping tarball.MultiRefWrite with a new file.
func MultiRefWriteToFile(p string, refToImage map[name.Reference]v1.Image) error {
	w, err := os.Create(p)
	if err != nil {
		return err
	}
	defer w.Close()

	return MultiRefWrite(refToImage, w)
}

// Write is a wrapper to write a single image in V1 format and tag to a tarball.
func Write(ref name.Reference, img v1.Image, w io.Writer) error {
	return MultiRefWrite(map[name.Reference]v1.Image{ref: img}, w)
}

// MultiWrite writes the contents of each image to the provided reader, in the V1 image tarball format.
// The contents are written in the following format:
// One manifest.json file at the top level containing information about several images.
// One repositories file mapping from the image <registry>/<repo name> to <tag> to the id of the top most layer.
// For every layer, a directory named with the layer ID is created with the following contents:
//   layer.tar - The uncompressed layer tarball.
//   <layer id>.json- Layer metadata json.
//   VERSION- Schema version string. Always set to "1.0".
// One file for the config blob, named after its SHA.
func MultiWrite(tagToImage map[name.Tag]v1.Image, w io.Writer) error {
	refToImage := make(map[name.Reference]v1.Image, len(tagToImage))
	for i, d := range tagToImage {
		refToImage[i] = d
	}
	return MultiRefWrite(refToImage, w)
}

// v1Layer represents a layer with metadata needed by the v1 image spec https://github.com/moby/moby/blob/master/image/spec/v1.md.
type v1Layer struct {
	// config is the layer metadata.
	config *legacy.LayerConfigFile
	// layer is the v1.Layer object this v1Layer represents.
	layer v1.Layer
}

// json returns the raw bytes of the json metadata of the given v1Layer.
func (l *v1Layer) json() ([]byte, error) {
	return json.Marshal(l.config)
}

// version returns the raw bytes of the "VERSION" file of the given v1Layer.
func (l *v1Layer) version() []byte {
	return []byte("1.0")
}

// v1LayerID computes the v1 image format layer id for the given v1.Layer with the given v1 parent ID and raw image config.
func v1LayerID(layer v1.Layer, parentID string, rawConfig []byte) (string, error) {
	d, err := layer.Digest()
	if err != nil {
		return "", errors.Wrap(err, "unable to get layer digest to generate v1 layer ID")
	}
	s := fmt.Sprintf("%s %s", d.Hex, parentID)
	if len(rawConfig) != 0 {
		s = fmt.Sprintf("%s %s", s, string(rawConfig))
	}
	rawDigest := sha256.Sum256([]byte(s))
	return hex.EncodeToString(rawDigest[:]), nil
}

// newTopV1Layer creates a new v1Layer for a layer other than the top layer in a v1 image tarball.
func newV1Layer(layer v1.Layer, parent *v1Layer, history v1.History) (*v1Layer, error) {
	parentID := ""
	if parent != nil {
		parentID = parent.config.ID
	}
	id, err := v1LayerID(layer, parentID, nil)
	if err != nil {
		return nil, errors.Wrap(err, "unable to generate v1 layer ID")
	}
	result := &v1Layer{
		layer: layer,
		config: &legacy.LayerConfigFile{
			ID:      id,
			Parent:  parentID,
			Created: history.Created,
			Comment: history.Comment,
			Author:  history.Author,
			ContainerConfig: v1.Config{
				Cmd: []string{history.CreatedBy},
			},
			Throwaway: history.EmptyLayer,
		},
	}
	return result, nil
}

// newTopV1Layer creates a new v1Layer for the top layer in a v1 image tarball.
func newTopV1Layer(layer v1.Layer, parent *v1Layer, history v1.History, imgConfig *v1.ConfigFile, rawConfig []byte) (*v1Layer, error) {
	result, err := newV1Layer(layer, parent, history)
	if err != nil {
		return nil, err
	}
	id, err := v1LayerID(layer, result.config.Parent, rawConfig)
	if err != nil {
		return nil, errors.Wrap(err, "unable to generate v1 layer ID for top layer")
	}
	result.config.ID = id
	result.config.Architecture = imgConfig.Architecture
	result.config.Container = imgConfig.Container
	result.config.DockerVersion = imgConfig.DockerVersion
	result.config.OS = imgConfig.OS
	result.config.Config = imgConfig.Config
	result.config.ContainerConfig = imgConfig.ContainerConfig
	result.config.Created = imgConfig.Created
	return result, nil
}

// addTags adds the given image tags to the given "repositories" file descriptor in a v1 image tarball.
func addTags(repos repositoriesTarDescriptor, tags []string, topLayerID string) {
	for _, t := range tags {
		base, tag := name.SplitTag(t)
		tagToID, ok := repos[base]
		if !ok {
			tagToID = make(map[string]string)
			repos[base] = tagToID
		}
		tagToID[tag] = topLayerID
	}
}

// updateLayerSources updates the given layer digest to descriptor map with the descriptor of the given layer in the given image if it's an undistributable layer.
func updateLayerSources(layerSources map[v1.Hash]v1.Descriptor, layer v1.Layer, img v1.Image) error {
	d, err := layer.Digest()
	if err != nil {
		return err
	}
	// Add to LayerSources if it's a foreign layer.
	desc, err := partial.BlobDescriptor(img, d)
	if err != nil {
		return err
	}
	if !desc.MediaType.IsDistributable() {
		diffid, err := partial.BlobToDiffID(img, d)
		if err != nil {
			return err
		}
		layerSources[diffid] = desc
	}
	return nil
}

// MultiRefWrite writes the contents of each image to the provided reader, in the V1 image tarball format.
// The contents are written in the following format:
// One manifest.json file at the top level containing information about several images.
// One repositories file mapping from the image <registry>/<repo name> to <tag> to the id of the top most layer.
// For every layer, a directory named with the layer ID is created with the following contents:
//   layer.tar - The uncompressed layer tarball.
//   <layer id>.json- Layer metadata json.
//   VERSION- Schema version string. Always set to "1.0".
// One file for the config blob, named after its SHA.
func MultiRefWrite(refToImage map[name.Reference]v1.Image, w io.Writer) error {
	tf := tar.NewWriter(w)
	defer tf.Close()

	imageToTags := dedupRefToImage(refToImage)
	var td tarDescriptor
	repos := make(repositoriesTarDescriptor)

	for img, tags := range imageToTags {
		// Write the config.
		cfgName, err := img.ConfigName()
		if err != nil {
			return err
		}
		cfgFileName := fmt.Sprintf("%s.json", cfgName.Hex)
		cfgBlob, err := img.RawConfigFile()
		if err != nil {
			return err
		}
		if err := writeTarEntry(tf, cfgFileName, bytes.NewReader(cfgBlob), int64(len(cfgBlob))); err != nil {
			return err
		}
		cfg, err := img.ConfigFile()
		if err != nil {
			return err
		}

		// Store foreign layer info.
		layerSources := make(map[v1.Hash]v1.Descriptor)

		// Write the layers.
		layers, err := img.Layers()
		if err != nil {
			return err
		}
		layerFiles := make([]string, len(layers))
		var prev *v1Layer
		for i, l := range layers {
			if err := updateLayerSources(layerSources, l, img); err != nil {
				return errors.Wrap(err, "unable to update image metadata to include undistributable layer source information")
			}
			var cur *v1Layer
			if i < (len(layers) - 1) {
				cur, err = newV1Layer(l, prev, cfg.History[i])
			} else {
				cur, err = newTopV1Layer(l, prev, cfg.History[i], cfg, cfgBlob)
			}
			if err != nil {
				return err
			}
			layerFiles[i] = fmt.Sprintf("%s/layer.tar", cur.config.ID)
			u, err := l.Uncompressed()
			if err != nil {
				return err
			}
			defer u.Close()
			// Reads the entire uncompressed blob into memory! Can be avoided
			// for some layer implementations where the uncompressed blob is
			// stored on disk and the layer can just stat the file.
			uncompressedBlob, err := ioutil.ReadAll(u)
			if err != nil {
				return err
			}
			if err := writeTarEntry(tf, layerFiles[i], bytes.NewReader(uncompressedBlob), int64(len(uncompressedBlob))); err != nil {
				return err
			}
			j, err := cur.json()
			if err != nil {
				return err
			}
			if err := writeTarEntry(tf, fmt.Sprintf("%s/json", cur.config.ID), bytes.NewReader(j), int64(len(j))); err != nil {
				return err
			}
			v := cur.version()
			if err := writeTarEntry(tf, fmt.Sprintf("%s/VERSION", cur.config.ID), bytes.NewReader(v), int64(len(v))); err != nil {
				return err
			}
			prev = cur
		}

		// Generate the tar descriptor and write it.
		sitd := singleImageTarDescriptor{
			Config:       cfgFileName,
			RepoTags:     tags,
			Layers:       layerFiles,
			LayerSources: layerSources,
		}

		td = append(td, sitd)
		// prev should be the top layer here. Use it to add the image tags
		// to the tarball repositories file.
		addTags(repos, tags, prev.config.ID)
	}

	tdBytes, err := json.Marshal(td)
	if err != nil {
		return err
	}
	if err := writeTarEntry(tf, "manifest.json", bytes.NewReader(tdBytes), int64(len(tdBytes))); err != nil {
		return err
	}
	reposBytes, err := json.Marshal(&repos)
	if err != nil {
		return err
	}
	if err := writeTarEntry(tf, "repositories", bytes.NewReader(reposBytes), int64(len(reposBytes))); err != nil {
		return err
	}
	return nil
}

func dedupRefToImage(refToImage map[name.Reference]v1.Image) map[v1.Image][]string {
	imageToTags := make(map[v1.Image][]string)

	for ref, img := range refToImage {
		if tag, ok := ref.(name.Tag); ok {
			if tags, ok := imageToTags[img]; ok && tags != nil {
				imageToTags[img] = append(tags, tag.String())
			} else {
				imageToTags[img] = []string{tag.String()}
			}
		} else {
			if _, ok := imageToTags[img]; !ok {
				imageToTags[img] = nil
			}
		}
	}

	return imageToTags
}

// write a file to the provided writer with a corresponding tar header
func writeTarEntry(tf *tar.Writer, path string, r io.Reader, size int64) error {
	hdr := &tar.Header{
		Mode:     0644,
		Typeflag: tar.TypeReg,
		Size:     size,
		Name:     path,
	}
	if err := tf.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := io.Copy(tf, r)
	return err
}
