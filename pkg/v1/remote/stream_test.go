package remote

import (
	"bytes"
	"crypto/rand"
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

func TestStreamableLayerZeroes(t *testing.T) {
	sl := NewStreamableLayer(ioutil.NopCloser(bytes.NewBufferString(strings.Repeat("a", n))))

	// All methods should return zero data and no errors until the stream
	// has been consumed.
	if size, err := sl.Size(); err != nil {
		t.Errorf("Size: %v", err)
	} else if size != 0 {
		t.Errorf("Size got %d, want 0", size)
	}
	if dig, err := sl.Digest(); err != nil {
		t.Errorf("Digest: %v", err)
	} else if dig.String() != (v1.Hash{}).String() {
		t.Errorf("Digest got %v, want %v", dig, v1.Hash{})
	}
	if diffID, err := sl.DiffID(); err != nil {
		t.Errorf("DiffID: %v", err)
	} else if diffID.String() != (v1.Hash{}).String() {
		t.Errorf("DiffID got %v, want %v", diffID, v1.Hash{})
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
