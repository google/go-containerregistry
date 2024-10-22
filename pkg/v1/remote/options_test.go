package remote

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
)

func TestOptionsInsecure(t *testing.T) {
	for _, targetType := range []string{
		"registry",
		"repository",
		"digest",
	} {
		for _, mode := range []string{
			"secure",
			"insecure",
		} {
			t.Run(targetType+"_"+mode, func(t *testing.T) {
				server := httptest.NewTLSServer(
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusOK)
					}))
				defer server.Close()

				u, err := url.Parse(server.URL)
				if err != nil {
					t.Fatal(err)
				}

				options := []name.Option{}

				if mode == "insecure" {
					options = append(options, name.Insecure)
				}

				var target resource

				switch targetType {
				case "registry":
					reg, err := name.NewRegistry("myregistry", options...)
					if err != nil {
						t.Fatal(err)
					}
					target = reg

				case "repository":
					ref, err := name.ParseReference("myregistry/name:tag", options...)
					if err != nil {
						t.Fatal(err)
					}
					target = ref.Context()

				case "digest":
					d, err := name.NewDigest("myregistry/name@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", options...)
					if err != nil {
						t.Fatal(err)
					}
					target = d
				}

				opts, err := makeOptions(target, []Option{}...)
				if err != nil {
					t.Fatal(err)
				}

				c := &http.Client{Transport: opts.transport}

				res, err := c.Get(u.String())

				if mode == "secure" {
					if ue, ok := err.(*url.Error); !ok {
						t.Fatal(err)
					} else if _, ok := ue.Err.(*tls.CertificateVerificationError); !ok {
						t.Fatal(err)
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}
					defer res.Body.Close()

					if res.StatusCode != http.StatusOK {
						t.Fatal(fmt.Printf("unexpected status code: %d", res.StatusCode))
					}
				}
			})
		}
	}
}
