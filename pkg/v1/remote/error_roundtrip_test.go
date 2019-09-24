package remote_test

import (
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

func TestStatusCodeReturned(t *testing.T) {
	tcs := []struct {
		Description string
		Handler     http.Handler
	}{{
		Description: "Only returns teapot status",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		}),
	}, {
		Description: "Handle v2, returns teapot status else",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Print(r.URL.Path)
			if r.URL.Path == "/v2/" {
				return
			}
			w.WriteHeader(http.StatusTeapot)
		}),
	}}
	for _, tc := range tcs {
		t.Run(tc.Description, func(t *testing.T) {
			o := httptest.NewServer(tc.Handler)
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
		})
	}
}
