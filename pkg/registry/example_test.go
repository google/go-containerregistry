package registry_test

import (
	"fmt"
	"net/http/httptest"

	"github.com/google/go-containerregistry/pkg/registry"
)

func Example() {
	s := httptest.NewServer(registry.New())
	defer s.Close()
	resp, _ := s.Client().Get(s.URL + "/v2/bar/blobs/sha256:...")
	fmt.Println(resp.StatusCode)
	// Output: 404
}
