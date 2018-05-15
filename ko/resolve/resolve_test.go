// Copyright 2018 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resolve

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/go-containerregistry/ko/build"
	"github.com/google/go-containerregistry/ko/publish"
	"github.com/google/go-containerregistry/name"
	"github.com/google/go-containerregistry/v1"
	"github.com/google/go-containerregistry/v1/random"
	yaml "gopkg.in/yaml.v2"
)

var (
	fooRef      = "github.com/awesomesauce/foo"
	foo         = mustRandom()
	fooHash     = mustDigest(foo)
	barRef      = "github.com/awesomesauce/bar"
	bar         = mustRandom()
	barHash     = mustDigest(bar)
	bazRef      = "github.com/awesomesauce/baz"
	baz         = mustRandom()
	bazHash     = mustDigest(baz)
	testBuilder = build.NewFixed(map[string]v1.Image{
		fooRef: foo,
		barRef: bar,
		bazRef: baz,
	})
	testHashes = map[string]v1.Hash{
		fooRef: fooHash,
		barRef: barHash,
		bazRef: bazHash,
	}
)

func mustReadAll(t *testing.T, rc io.ReadCloser) []byte {
	t.Helper()
	all, err := ioutil.ReadAll(rc)
	if err != nil {
		t.Errorf("Error reading all: %v", err)
	}
	if err := rc.Close(); err != nil {
		t.Errorf("Error closing after read: %v", err)
	}
	return all
}

func TestYAMLArrays(t *testing.T) {
	tests := []struct {
		desc   string
		refs   []string
		hashes []v1.Hash
		base   name.Repository
	}{{
		desc:   "singleton array",
		refs:   []string{fooRef},
		hashes: []v1.Hash{fooHash},
		base:   mustRepository("gcr.io/mattmoor"),
	}, {
		desc:   "singleton array (different base)",
		refs:   []string{fooRef},
		hashes: []v1.Hash{fooHash},
		base:   mustRepository("gcr.io/jasonhall"),
	}, {
		desc:   "two element array",
		refs:   []string{fooRef, barRef},
		hashes: []v1.Hash{fooHash, barHash},
		base:   mustRepository("gcr.io/jonjohnson"),
	}, {
		desc:   "empty array",
		refs:   []string{},
		hashes: []v1.Hash{},
		base:   mustRepository("gcr.io/blah"),
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			inputStructured := test.refs
			inputYAML, err := yaml.Marshal(inputStructured)
			if err != nil {
				t.Fatalf("yaml.Marshal(%v) = %v", inputStructured, err)
			}

			outYAML := mustReadAll(t, ImageReferences(bytes.NewReader(inputYAML), testBuilder, publish.NewFixed(test.base, testHashes)))
			var outStructured []string
			if err := yaml.Unmarshal(outYAML, &outStructured); err != nil {
				t.Errorf("yaml.Unmarshal(%v) = %v", string(outYAML), err)
			}

			if want, got := len(inputStructured), len(outStructured); want != got {
				t.Errorf("ImageReferences(%v) = %v, want %v", string(inputYAML), got, want)
			}

			var expectedStructured []string
			for i, ref := range test.refs {
				hash := test.hashes[i]
				expectedStructured = append(expectedStructured,
					computeDigest(test.base, ref, hash).String())
			}

			if diff := cmp.Diff(expectedStructured, outStructured, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("ImageReferences(%v); (-want +got) = %v", string(inputYAML), diff)
			}
		})
	}
}

func TestYAMLMaps(t *testing.T) {
	base := mustRepository("gcr.io/mattmoor")
	tests := []struct {
		desc     string
		input    map[string]string
		expected map[string]string
	}{{
		desc:     "simple value",
		input:    map[string]string{"image": fooRef},
		expected: map[string]string{"image": computeDigest(base, fooRef, fooHash).String()},
	}, {
		desc:  "simple key",
		input: map[string]string{bazRef: "blah"},
		expected: map[string]string{
			computeDigest(base, bazRef, bazHash).String(): "blah",
		},
	}, {
		desc:  "key and value",
		input: map[string]string{fooRef: barRef},
		expected: map[string]string{
			computeDigest(base, fooRef, fooHash).String(): computeDigest(base, barRef, barHash).String(),
		},
	}, {
		desc:     "empty map",
		input:    map[string]string{},
		expected: map[string]string{},
	}, {
		desc: "multiple values",
		input: map[string]string{
			"arg1": fooRef,
			"arg2": barRef,
		},
		expected: map[string]string{
			"arg1": computeDigest(base, fooRef, fooHash).String(),
			"arg2": computeDigest(base, barRef, barHash).String(),
		},
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			inputStructured := test.input
			inputYAML, err := yaml.Marshal(inputStructured)
			if err != nil {
				t.Fatalf("yaml.Marshal(%v) = %v", inputStructured, err)
			}

			outYAML := mustReadAll(t, ImageReferences(bytes.NewReader(inputYAML), testBuilder, publish.NewFixed(base, testHashes)))
			var outStructured map[string]string
			if err := yaml.Unmarshal(outYAML, &outStructured); err != nil {
				t.Errorf("yaml.Unmarshal(%v) = %v", string(outYAML), err)
			}

			if want, got := len(inputStructured), len(outStructured); want != got {
				t.Errorf("ImageReferences(%v) = %v, want %v", string(inputYAML), got, want)
			}

			if diff := cmp.Diff(test.expected, outStructured, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("ImageReferences(%v); (-want +got) = %v", string(inputYAML), diff)
			}
		})
	}
}

// object has public fields to avoid `yaml:"foo"` annotations.
type object struct {
	S string
	M map[string]object
	A []object
	P *object
}

func TestYAMLObject(t *testing.T) {
	base := mustRepository("gcr.io/bazinga")
	tests := []struct {
		desc     string
		input    *object
		expected *object
	}{{
		desc:     "empty object",
		input:    &object{},
		expected: &object{},
	}, {
		desc:     "string field",
		input:    &object{S: fooRef},
		expected: &object{S: computeDigest(base, fooRef, fooHash).String()},
	}, {
		desc:     "map field",
		input:    &object{M: map[string]object{"blah": object{S: fooRef}}},
		expected: &object{M: map[string]object{"blah": object{S: computeDigest(base, fooRef, fooHash).String()}}},
	}, {
		desc:     "array field",
		input:    &object{A: []object{{S: fooRef}}},
		expected: &object{A: []object{{S: computeDigest(base, fooRef, fooHash).String()}}},
	}, {
		desc:     "pointer field",
		input:    &object{P: &object{S: fooRef}},
		expected: &object{P: &object{S: computeDigest(base, fooRef, fooHash).String()}},
	}, {
		desc:     "deep field",
		input:    &object{M: map[string]object{"blah": object{A: []object{{P: &object{S: fooRef}}}}}},
		expected: &object{M: map[string]object{"blah": object{A: []object{{P: &object{S: computeDigest(base, fooRef, fooHash).String()}}}}}},
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			inputStructured := test.input
			inputYAML, err := yaml.Marshal(inputStructured)
			if err != nil {
				t.Fatalf("yaml.Marshal(%v) = %v", inputStructured, err)
			}

			outYAML := mustReadAll(t, ImageReferences(bytes.NewReader(inputYAML), testBuilder, publish.NewFixed(base, testHashes)))
			var outStructured *object
			if err := yaml.Unmarshal(outYAML, &outStructured); err != nil {
				t.Errorf("yaml.Unmarshal(%v) = %v", string(outYAML), err)
			}

			if diff := cmp.Diff(test.expected, outStructured, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("ImageReferences(%v); (-want +got) = %v", string(inputYAML), diff)
			}
		})
	}
}

func TestMultiDocumentYAMLs(t *testing.T) {
	tests := []struct {
		desc   string
		refs   []string
		hashes []v1.Hash
		base   name.Repository
	}{{
		desc:   "two string documents",
		refs:   []string{fooRef, barRef},
		hashes: []v1.Hash{fooHash, barHash},
		base:   mustRepository("gcr.io/multi-pass"),
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			buf := bytes.NewBuffer(nil)
			encoder := yaml.NewEncoder(buf)
			for _, input := range test.refs {
				if err := encoder.Encode(input); err != nil {
					t.Fatalf("Encode(%v) = %v", input, err)
				}
			}
			inputYAML := buf.Bytes()

			outYAML := mustReadAll(t, ImageReferences(bytes.NewReader(inputYAML), testBuilder, publish.NewFixed(test.base, testHashes)))

			buf = bytes.NewBuffer(outYAML)
			decoder := yaml.NewDecoder(buf)
			var outStructured []string
			for {
				var output string
				if err := decoder.Decode(&output); err == nil {
					outStructured = append(outStructured, output)
				} else if err == io.EOF {
					outStructured = append(outStructured, output)
					break
				} else {
					t.Errorf("yaml.Unmarshal(%v) = %v", string(outYAML), err)
				}
			}

			var expectedStructured []string
			for i, ref := range test.refs {
				hash := test.hashes[i]
				expectedStructured = append(expectedStructured,
					computeDigest(test.base, ref, hash).String())
			}
			// The multi-document output always seems to leave a trailing --- so we end up with
			// an extra empty element.
			expectedStructured = append(expectedStructured, "")

			if want, got := len(expectedStructured), len(outStructured); want != got {
				t.Errorf("ImageReferences(%v) = %v, want %v", string(inputYAML), got, want)
			}

			if diff := cmp.Diff(expectedStructured, outStructured, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("ImageReferences(%v); (-want +got) = %v", string(inputYAML), diff)
			}
		})
	}
}

// TestImageReferencesError tests that errors encountered during resolution are
// exposed to the ReadCloser's Read method, except when the error is EOF in
// which case the error is ignored.
func TestImageReferencesError(t *testing.T) {
	rc := ImageReferences(strings.NewReader("}{}{invalid yaml"), nil, nil)
	if _, err := io.Copy(ioutil.Discard, rc); err == nil {
		t.Errorf("Read: got no error, expected error")
	}
	if err := rc.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}

	rc = ImageReferences(strings.NewReader(""), nil, nil)
	if _, err := io.Copy(ioutil.Discard, rc); err != nil {
		t.Errorf("Read empty YAML: got %v, want nil (EOF)", err)
	}
	if err := rc.Close(); err != nil {
		t.Errorf("Close empty YAML: got %v, want nil", err)
	}
}

// TestImageReferencesErrorPublishing tests that failures to build are surfaced
// as errors in the returned ReadCloser.
func TestImageReferencesErrorBuilding(t *testing.T) {
	inputYAML, err := yaml.Marshal([]string{fooRef, barRef, bazRef, "github.com/awesomesauce/badbadbad"})
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}

	rc := ImageReferences(bytes.NewReader(inputYAML), badBuilder{}, publish.NewFixed(mustRepository("gcr.io/jasonhall"), testHashes))
	if _, err := ioutil.ReadAll(rc); err == nil {
		t.Errorf("Read got no error, expected error")
	}
}

// TestImageReferencesErrorPublishing tests that failures to publish are
// surfaced as errors in the returned ReadCloser.
func TestImageReferencesErrorPublishing(t *testing.T) {
	inputYAML, err := yaml.Marshal([]string{fooRef, barRef, bazRef})
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}

	rc := ImageReferences(bytes.NewReader(inputYAML), testBuilder, badPublisher{})
	if _, err := ioutil.ReadAll(rc); err == nil {
		t.Errorf("Read got no error, expected error")
	}
}

type badBuilder struct{}

func (badBuilder) IsSupportedReference(string) bool { return true }
func (badBuilder) Build(s string) (v1.Image, error) {
	if strings.Contains(s, "bad") {
		return nil, errors.New("cannot build")
	}
	return random.Image(10, 10)
}

type badPublisher struct{}

func (badPublisher) Publish(i v1.Image, s string) (*name.Digest, error) {
	return nil, errors.New("cannot publish")
}

func mustRandom() v1.Image {
	img, err := random.Image(5, 1024)
	if err != nil {
		panic(err)
	}
	return img
}

func mustRepository(s string) name.Repository {
	n, err := name.NewRepository(s, name.WeakValidation)
	if err != nil {
		panic(err)
	}
	return n
}

func mustDigest(img v1.Image) v1.Hash {
	d, err := img.Digest()
	if err != nil {
		panic(err)
	}
	return d
}

func computeDigest(base name.Repository, ref string, h v1.Hash) name.Digest {
	d, err := publish.NewFixed(base, map[string]v1.Hash{ref: h}).Publish(nil, ref)
	if err != nil {
		panic(err)
	}
	return *d
}
