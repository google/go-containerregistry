package remote

import (
	"archive/tar"
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1"
)

const (
	n          = 10000
	wantDigest = "sha256:4aaecd841543b43d8815fa086939ee183462c1fea721a16fd28c825aa1af571f"
	wantDiffID = "sha256:27dd1f61b867b6a0f6e9d8a41c43231de52107e53ae424de8f847b821db4b711"
)

func TestStreamableLayerZero(t *testing.T) {
	sl := NewStreamableLayer(ioutil.NopCloser(bytes.NewBufferString(strings.Repeat("a", n))))

	// All methods should return errors until the stream has been consumed and closed.
	if _, err := sl.Size(); err == nil {
		t.Error("Size: got nil error, wanted error")
	}
	if _, err := sl.Digest(); err == nil {
		t.Error("Digest: got nil error, wanted error")
	}
	if _, err := sl.DiffID(); err == nil {
		t.Error("DiffID: got nil error, wanted error")
	}
}

func TestStreamableLayerCompressed(t *testing.T) {
	sl := NewStreamableLayer(ioutil.NopCloser(bytes.NewBufferString(strings.Repeat("a", n))))

	// This will consume the originally uncompressed data and produce
	// compressed data, and closing will populate the StreamableLayer's
	// digest, diffID and size.
	rc, err := sl.Compressed()
	if err != nil {
		t.Fatalf("Compressed: %v", err)
	}
	if _, err := io.Copy(ioutil.Discard, rc); err != nil {
		t.Fatalf("Reading layer: %v", err)
	}
	if err := rc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Now that the stream has been consumed, data should be available.
	if dig, err := sl.Digest(); err != nil {
		t.Errorf("Digest: %v", err)
	} else if dig.String() != wantDigest {
		t.Errorf("Digest got %q, want %q", dig, wantDigest)
	}
	if diffID, err := sl.DiffID(); err != nil {
		t.Errorf("DiffID: %v", err)
	} else if diffID.String() != wantDiffID {
		t.Errorf("DiffID got %q, want %q", diffID, wantDiffID)
	}
	if size, err := sl.Size(); err != nil {
		t.Errorf("Size: %v", err)
	} else if size != int64(n) {
		t.Errorf("Size got %d, want %d", size, n)
	}
}

func TestStreamableLayerUncompressed(t *testing.T) {
	sl := NewStreamableLayer(ioutil.NopCloser(bytes.NewBufferString(strings.Repeat("a", n))))

	// This will consume the given ReadCloser, and closing will populate
	// the StreamableLayer's digest, diffID and size.
	rc, err := sl.Uncompressed()
	if err != nil {
		t.Fatalf("Compressed: %v", err)
	}
	if _, err := io.Copy(ioutil.Discard, rc); err != nil {
		t.Fatalf("Reading layer: %v", err)
	}
	if err := rc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Now that the stream has been consumed, data should be available.
	if dig, err := sl.Digest(); err != nil {
		t.Errorf("Digest: %v", err)
	} else if dig.String() != wantDigest {
		t.Errorf("Digest got %q, want %q", dig, wantDigest)
	}
	if diffID, err := sl.DiffID(); err != nil {
		t.Errorf("DiffID: %v", err)
	} else if diffID.String() != wantDiffID {
		t.Errorf("DiffID got %q, want %q", diffID, wantDiffID)
	}
	if size, err := sl.Size(); err != nil {
		t.Errorf("Size: %v", err)
	} else if size != int64(n) {
		t.Errorf("Size got %d, want %d", size, n)
	}
}

// Streaming a huge random layer through StreamableLayer computes its
// digest/diffID/size, without buffering.
func TestLargeStreamedLayer(t *testing.T) {
	n := int64(100000000)
	sl := NewStreamableLayer(ioutil.NopCloser(io.LimitReader(rand.Reader, n)))
	rc, err := sl.Uncompressed()
	if err != nil {
		t.Fatalf("Uncompressed: %v", err)
	}
	if _, err := io.Copy(ioutil.Discard, rc); err != nil {
		t.Fatalf("Reading layer: %v", err)
	}
	if err := rc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if dig, err := sl.Digest(); err != nil {
		t.Errorf("Digest: %v", err)
	} else if dig.String() == (v1.Hash{}).String() {
		t.Errorf("Digest got %q, want anything else", (v1.Hash{}).String())
	}
	if diffID, err := sl.DiffID(); err != nil {
		t.Errorf("DiffID: %v", err)
	} else if diffID.String() == (v1.Hash{}).String() {
		t.Errorf("DiffID got %q, want anything else", (v1.Hash{}).String())
	}
	if size, err := sl.Size(); err != nil {
		t.Errorf("Size: %v", err)
	} else if size != n {
		t.Errorf("Size got %d, want %d", size, n)
	}
}

func TestStreamableLayerFromTarball(t *testing.T) {
	pr, pw := io.Pipe()
	tw := tar.NewWriter(pw)

	go func() {
		// "Stream" a bunch of files into the layer.
		pw.CloseWithError(func() error {
			for i := 0; i < 1000; i++ {
				name := fmt.Sprintf("file-%d.txt", i)
				body := fmt.Sprintf("i am file number %d", i)
				if err := tw.WriteHeader(&tar.Header{
					Name: name,
					Mode: 0600,
					Size: int64(len(body)),
				}); err != nil {
					return err
				}
				if _, err := tw.Write([]byte(body)); err != nil {
					return err
				}
			}
			return nil
		}())
	}()

	sl := NewStreamableLayer(pr)
	rc, err := sl.Compressed()
	if err != nil {
		t.Fatalf("Compressed: %v", err)
	}
	if _, err := io.Copy(ioutil.Discard, rc); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if err := rc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	wantDigest := "sha256:a50b9da8ea0a16cc6397af5cbb134ecbcd6e673fad3ea0a85b14146aae7de793"
	got, err := sl.Digest()
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if got.String() != wantDigest {
		t.Errorf("Digest: got %q, want %q", got.String(), wantDigest)
	}
}
