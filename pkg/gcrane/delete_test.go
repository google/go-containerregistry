// Copyright 2020 Google LLC All Rights Reserved.
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

package gcrane

import (
	"net/http"
	"os"
	"testing"

	ggcrtest "github.com/google/go-containerregistry/pkg/internal/httptest"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func TestDelete(t *testing.T) {
	logs.Warn.SetOutput(os.Stderr)
	refStr := "gcr.io/test/gcrane"

	deleteImg, err := random.Image(1024, 2)
	if err != nil {
		t.Fatal(err)
	}
	notExistImg, err := random.Image(1024, 2)
	if err != nil {
		t.Fatal(err)
	}

	latestRef, err := name.ParseReference(refStr)
	if err != nil {
		t.Fatal(err)
	}
	d, err := deleteImg.Digest()
	if err != nil {
		t.Fatal(err)
	}
	deleteRef := latestRef.Context().Digest(d.String())
	d2, err := notExistImg.Digest()
	if err != nil {
		t.Fatal(err)
	}
	notExistRef := latestRef.Context().Digest(d2.String())

	h, err := newFakeGCR(map[name.Reference]partial.Describable{
		deleteRef: deleteImg,
	}, t)
	if err != nil {
		t.Fatal(err)
	}
	s, err := ggcrtest.NewTLSServer("gcr.io", h)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Make sure we don't actually talk to GCR.
	http.DefaultTransport = s.Client().Transport

	if err := remote.Write(deleteRef, deleteImg); err != nil {
		t.Fatal(err)
	}

	if err := Delete(notExistRef.Name()); err == nil {
		t.Fatal("Not exist reference shouldn't be accepted")
	}
	if err := Delete(deleteRef.Name()); err != nil {
		t.Fatal(err)
	}
	if err := Delete(deleteRef.Name()); err == nil {
		t.Fatal("Already deleted reference shouldn't be accepted")
	}
}
