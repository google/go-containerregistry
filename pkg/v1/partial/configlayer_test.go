package partial

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type testUIC struct {
	configFile []byte
}

func (t testUIC) RawConfigFile() ([]byte, error) {
	return t.configFile, nil
}

func (t testUIC) MediaType() (types.MediaType, error) {
	return "", nil
}

func (t testUIC) LayerByDiffID(h v1.Hash) (UncompressedLayer, error) {
	return nil, fmt.Errorf("no layer by diff ID %v", h)
}

func TestUncompressedLayerByDigest(t *testing.T) {
	uic := testUIC{
		configFile: []byte("{}"),
	}
	uie := &uncompressedImageExtender{
		UncompressedImageCore: uic,
	}
	testLayerByDigestForConfigHash(t, uie)
}

type testCIC struct {
	configFile []byte
}

func (t testCIC) RawConfigFile() ([]byte, error) {
	return t.configFile, nil
}

func (t testCIC) MediaType() (types.MediaType, error) {
	return "", nil
}

func (testCIC) RawManifest() ([]byte, error) {
	return nil, nil
}

func (t testCIC) LayerByDigest(h v1.Hash) (CompressedLayer, error) {
	return nil, fmt.Errorf("no layer by diff ID %v", h)
}

func TestCompressedLayersByDigest(t *testing.T) {
	cic := testCIC{
		configFile: []byte("{}"),
	}
	cie := &compressedImageExtender{
		CompressedImageCore: cic,
	}
	testLayerByDigestForConfigHash(t, cie)
}

func testLayerByDigestForConfigHash(t *testing.T, image v1.Image) {
	hash, err := image.ConfigName()
	if err != nil {
		t.Fatalf("Error getting config name: %v", err)
	}

	layer, err := image.LayerByDigest(hash)
	if err != nil {
		t.Fatalf("Error getting layer by digest: %v", err)
	}

	lr, err := layer.Uncompressed()
	if err != nil {
		t.Fatalf("Error getting uncompressed layer: %v", err)
	}

	cfgLayerBytes, err := ioutil.ReadAll(lr)
	if err != nil {
		t.Fatalf("Error reading config layer bytes: %v", err)
	}

	cfgFile, err := image.RawConfigFile()
	if err != nil {
		t.Fatalf("Error getting raw config file: %v", err)
	}

	if string(cfgFile) != string(cfgLayerBytes) {
		t.Errorf("Config file layer doesn't match raw config file")
	}
}
