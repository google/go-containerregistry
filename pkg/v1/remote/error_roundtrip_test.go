package remote_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

func TestStatusCodeReturned(t *testing.T) {
	o := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	defer o.Close()

	ref, err := name.NewDigest(strings.TrimPrefix(o.URL+"/foo:@sha256:53b27244ffa2f585799adbfaf79fba5a5af104597751b289c8b235e7b8f7ebf5", "http://"))
	if err != nil {
		t.Fatalf("Unable to parse digest: %v", err)
	}

	_, err = remote.Image(ref)
	terr, ok := err.(*transport.Error)
	if !ok {
		t.Fatalf("Unable to cast error to transport error: %v", err)
	}
	if terr.StatusCode != http.StatusTeapot {
		t.Errorf("Incorrect status code received, got %v, wanted %v", terr.StatusCode, http.StatusTeapot)
	}
}
