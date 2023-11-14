package layout

import (
	"path/filepath"
	"testing"
)

var (
	gcIndexPath              = filepath.Join("testdata", "test_gc_index")
	gcIndexBlobHash          = "sha256:492b89b9dd3cda4596f94916d17f6901455fb8bd7f4c5a2a90df8d39c90f48a0"
	gcUnknownMediaTypePath   = filepath.Join("testdata", "test_gc_image_unknown_mediatype")
	gcUnknownMediaTypeErr    = "gc: unknown media type: application/vnd.oci.descriptor.v1+json"
	gcTestOneImagePath       = filepath.Join("testdata", "test_index_one_image")
	gcTestIndexMediaTypePath = filepath.Join("testdata", "test_index_media_type")
)

func TestGcIndex(t *testing.T) {
	lp, err := FromPath(gcIndexPath)
	if err != nil {
		t.Fatalf("FromPath() = %v", err)
	}

	removed, err := lp.GarbageCollect()
	if err != nil {
		t.Fatalf("GarbageCollect() = %v", err)
	}

	if len(removed) != 1 {
		t.Fatalf("expected to have only one gc-able blob")
	}
	if removed[0].String() != gcIndexBlobHash {
		t.Fatalf("wrong blob is gc-ed: expected '%s', got '%s'", gcIndexBlobHash, removed[0].String())
	}
}

func TestGcOneImage(t *testing.T) {
	lp, err := FromPath(gcTestOneImagePath)
	if err != nil {
		t.Fatalf("FromPath() = %v", err)
	}

	removed, err := lp.GarbageCollect()
	if err != nil {
		t.Fatalf("GarbageCollect() = %v", err)
	}

	if len(removed) != 0 {
		t.Fatalf("expected to have to gc-able blobs")
	}
}

func TestGcIndexMediaType(t *testing.T) {
	lp, err := FromPath(gcTestIndexMediaTypePath)
	if err != nil {
		t.Fatalf("FromPath() = %v", err)
	}

	removed, err := lp.GarbageCollect()
	if err != nil {
		t.Fatalf("GarbageCollect() = %v", err)
	}

	if len(removed) != 0 {
		t.Fatalf("expected to have to gc-able blobs")
	}
}

func TestGcUnknownMediaType(t *testing.T) {
	lp, err := FromPath(gcUnknownMediaTypePath)
	if err != nil {
		t.Fatalf("FromPath() = %v", err)
	}

	_, err = lp.GarbageCollect()
	if err == nil {
		t.Fatalf("expected GarbageCollect to return err but did not")
	}

	if err.Error() != gcUnknownMediaTypeErr {
		t.Fatalf("expected error '%s', got '%s'", gcUnknownMediaTypeErr, err.Error())
	}
}
