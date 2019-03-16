package remote

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
)

func TestCheckPushPermission(t *testing.T) {
	for _, c := range []struct {
		status  int
		wantErr bool
	}{{
		http.StatusCreated,
		false,
	}, {
		http.StatusForbidden,
		true,
	}, {
		http.StatusBadRequest,
		true,
	}} {

		expectedRepo := "write/time"
		initiatePath := fmt.Sprintf("/v2/%s/blobs/uploads/", expectedRepo)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/v2/":
				w.WriteHeader(http.StatusOK)
			case initiatePath:
				if r.Method != http.MethodPost {
					t.Errorf("Method; got %v, want %v", r.Method, http.MethodPost)
				}
				w.Header().Set("Location", "somewhere/else")
				http.Error(w, "", c.status)
			default:
				t.Fatalf("Unexpected path: %v", r.URL.Path)
			}
		}))
		defer server.Close()
		u, err := url.Parse(server.URL)
		if err != nil {
			t.Fatalf("url.Parse(%v) = %v", server.URL, err)
		}

		ref := mustNewTag(t, fmt.Sprintf("%s/%s:latest", u.Host, expectedRepo))
		if err := CheckPushPermission(ref, authn.DefaultKeychain, http.DefaultTransport); (err != nil) != c.wantErr {
			t.Errorf("CheckPermission(%d): got error = %v, want err = %t", c.status, err, c.wantErr)
		}
	}
}
