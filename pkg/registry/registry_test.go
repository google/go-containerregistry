package registry_test

import (
	"net/http/httptest"
	"testing"

	"github.com/google/go-containerregistry/pkg/registry"
)

func TestCalls(t *testing.T) {
	tcs := []struct {
		Description string
		URL         string
		Code        int
	}{
		{
			Description: "/v2 returns 200",
			URL:         "/v2",
			Code:        200,
		},
		{
			Description: "/v2/ returns 200",
			URL:         "/v2/",
			Code:        200,
		},
		{
			Description: "/v2/bad returns 404",
			URL:         "/v2/bad",
			Code:        404,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Description, func(t *testing.T) {
			s := httptest.NewServer(registry.New())
			defer s.Close()
			resp, err := s.Client().Get(s.URL + tc.URL)
			if err != nil {
				t.Fatalf("Error getting %q: %v", tc.URL, err)
			}
			if resp.StatusCode != tc.Code {
				t.Errorf("Incorrect status code, got %d, want %d", resp.StatusCode, tc.Code)
			}
		})
	}
}
