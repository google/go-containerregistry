package test_test

import (
	"testing"

	"github.com/google/go-containerregistry/internal/test"
)

func TestTest(t *testing.T) {
	must := test.T(t)

	t.Log(must.Digest(must.RandomImage(1024, 5)))
	t.Log(must.Digest(must.RandomIndex(1024, 5, 5)))
	s := "gcr.io/foo/bar"
	t.Log(must.ParseReference(s))
	t.Log(must.Tag(s))
}
