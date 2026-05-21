package remote_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func TestReferrersSubjectBinding(t *testing.T) {
	t.Parallel()

	subjectDigest := "sha256:" + strings.Repeat("a", 64)
	otherSubjectDigest := "sha256:" + strings.Repeat("b", 64)

	makeReferrerManifest := func(subject string) ([]byte, v1.Hash) {
		t.Helper()
		m := map[string]any{
			"schemaVersion": 2,
			"mediaType":     string(types.OCIManifestSchema1),
			"config": map[string]any{
				"mediaType": string(types.OCIConfigJSON),
				"size":      2,
				"digest":    "sha256:" + strings.Repeat("c", 64),
			},
			"layers": []any{},
			"subject": map[string]any{
				"mediaType": string(types.OCIManifestSchema1),
				"size":      1,
				"digest":    subject,
			},
		}
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		h, _, err := v1.SHA256(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		return b, h
	}

	makeReferrerManifestNoSubject := func() ([]byte, v1.Hash) {
		t.Helper()
		m := map[string]any{
			"schemaVersion": 2,
			"mediaType":     string(types.OCIManifestSchema1),
			"config": map[string]any{
				"mediaType": string(types.OCIConfigJSON),
				"size":      2,
				"digest":    "sha256:" + strings.Repeat("d", 64),
			},
			"layers": []any{},
		}
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		h, _, err := v1.SHA256(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		return b, h
	}

	wrongManifest, wrongDigest := makeReferrerManifest(otherSubjectDigest)
	rightManifest, rightDigest := makeReferrerManifest(subjectDigest)
	nilManifest, nilDigest := makeReferrerManifestNoSubject()

	indexBytes, err := json.Marshal(map[string]any{
		"schemaVersion": 2,
		"mediaType":     string(types.OCIImageIndex),
		"subject": map[string]any{
			"mediaType": string(types.OCIManifestSchema1),
			"size":      0,
			"digest":    subjectDigest,
		},
		"manifests": []any{
			map[string]any{
				"mediaType":    string(types.OCIManifestSchema1),
				"size":         len(wrongManifest),
				"digest":       wrongDigest.String(),
				"artifactType": "application/testing123",
			},
			map[string]any{
				"mediaType":    string(types.OCIManifestSchema1),
				"size":         len(nilManifest),
				"digest":       nilDigest.String(),
				"artifactType": "application/testing123",
			},
			map[string]any{
				"mediaType":    string(types.OCIManifestSchema1),
				"size":         len(rightManifest),
				"digest":       rightDigest.String(),
				"artifactType": "application/testing123",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v2/":
			w.WriteHeader(http.StatusOK)
			return
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v2/repo/referrers/"):
			w.Header().Set("Content-Type", string(types.OCIImageIndex))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(indexBytes)
			return
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v2/repo/manifests/"):
			identifier := strings.TrimPrefix(r.URL.Path, "/v2/repo/manifests/")
			var body []byte
			switch identifier {
			case wrongDigest.String():
				body = wrongManifest
			case nilDigest.String():
				body = nilManifest
			case rightDigest.String():
				body = rightManifest
			default:
				http.NotFound(w, r)
				return
			}

			h, _, err := v1.SHA256(bytes.NewReader(body))
			if err != nil {
				t.Fatalf("SHA256: %v", err)
			}

			w.Header().Set("Content-Type", string(types.OCIManifestSchema1))
			w.Header().Set("Docker-Content-Digest", h.String())
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
			return
		default:
			http.NotFound(w, r)
			return
		}
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := name.NewRepository(u.Host+"/repo", name.Insecure)
	if err != nil {
		t.Fatal(err)
	}
	subjectRef := repo.Digest(subjectDigest)

	idx, err := remote.Referrers(subjectRef, remote.WithContext(context.Background()))
	if err != nil {
		t.Fatal(err)
	}
	im, err := idx.IndexManifest()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(im.Manifests), 1; got != want {
		t.Fatalf("len(IndexManifest().Manifests)=%d, want %d", got, want)
	}
	if got, want := im.Manifests[0].Digest.String(), rightDigest.String(); got != want {
		t.Fatalf("manifest[0].digest=%q, want %q", got, want)
	}
}

func TestReferrersSubjectBindingIndexReferrer(t *testing.T) {
	t.Parallel()

	subjectDigest := "sha256:" + strings.Repeat("a", 64)

	makeReferrerIndex := func(subject string) ([]byte, v1.Hash) {
		t.Helper()
		m := map[string]any{
			"schemaVersion": 2,
			"mediaType":     string(types.OCIImageIndex),
			"subject": map[string]any{
				"mediaType": string(types.OCIManifestSchema1),
				"size":      1,
				"digest":    subject,
			},
			"manifests": []any{},
		}
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		h, _, err := v1.SHA256(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		return b, h
	}

	indexReferrerBytes, indexReferrerDigest := makeReferrerIndex(subjectDigest)

	referrersIndexBytes, err := json.Marshal(map[string]any{
		"schemaVersion": 2,
		"mediaType":     string(types.OCIImageIndex),
		"subject": map[string]any{
			"mediaType": string(types.OCIManifestSchema1),
			"size":      0,
			"digest":    subjectDigest,
		},
		"manifests": []any{
			map[string]any{
				"mediaType":    string(types.OCIImageIndex),
				"size":         len(indexReferrerBytes),
				"digest":       indexReferrerDigest.String(),
				"artifactType": "application/testing123",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v2/":
			w.WriteHeader(http.StatusOK)
			return
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v2/repo/referrers/"):
			w.Header().Set("Content-Type", string(types.OCIImageIndex))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(referrersIndexBytes)
			return
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v2/repo/manifests/"):
			identifier := strings.TrimPrefix(r.URL.Path, "/v2/repo/manifests/")
			if identifier != indexReferrerDigest.String() {
				http.NotFound(w, r)
				return
			}
			h, _, err := v1.SHA256(bytes.NewReader(indexReferrerBytes))
			if err != nil {
				t.Fatalf("SHA256: %v", err)
			}
			w.Header().Set("Content-Type", string(types.OCIImageIndex))
			w.Header().Set("Docker-Content-Digest", h.String())
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(indexReferrerBytes)
			return
		default:
			http.NotFound(w, r)
			return
		}
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := name.NewRepository(u.Host+"/repo", name.Insecure)
	if err != nil {
		t.Fatal(err)
	}
	subjectRef := repo.Digest(subjectDigest)

	idx, err := remote.Referrers(subjectRef, remote.WithContext(context.Background()))
	if err != nil {
		t.Fatal(err)
	}
	im, err := idx.IndexManifest()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(im.Manifests), 1; got != want {
		t.Fatalf("len(IndexManifest().Manifests)=%d, want %d", got, want)
	}
	if got, want := im.Manifests[0].Digest.String(), indexReferrerDigest.String(); got != want {
		t.Fatalf("manifest[0].digest=%q, want %q", got, want)
	}
}

func TestReferrersIndexSubjectMismatchErrors(t *testing.T) {
	t.Parallel()

	subjectDigest := "sha256:" + strings.Repeat("a", 64)
	otherSubjectDigest := "sha256:" + strings.Repeat("b", 64)

	indexBytes, err := json.Marshal(map[string]any{
		"schemaVersion": 2,
		"mediaType":     string(types.OCIImageIndex),
		"subject": map[string]any{
			"mediaType": string(types.OCIManifestSchema1),
			"size":      0,
			"digest":    otherSubjectDigest,
		},
		"manifests": []any{},
	})
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v2/":
			w.WriteHeader(http.StatusOK)
			return
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v2/repo/referrers/"):
			w.Header().Set("Content-Type", string(types.OCIImageIndex))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(indexBytes)
			return
		default:
			http.NotFound(w, r)
			return
		}
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := name.NewRepository(u.Host+"/repo", name.Insecure)
	if err != nil {
		t.Fatal(err)
	}
	subjectRef := repo.Digest(subjectDigest)

	if _, err := remote.Referrers(subjectRef, remote.WithContext(context.Background())); err == nil {
		t.Fatalf("expected error, got nil")
	}
}
