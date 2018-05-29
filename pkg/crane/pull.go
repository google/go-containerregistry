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

package crane

import (
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/spf13/cobra"

	"github.com/google/go-containerregistry/authn"
	"github.com/google/go-containerregistry/name"
	"github.com/google/go-containerregistry/v1"
	"github.com/google/go-containerregistry/v1/remote"
	"github.com/google/go-containerregistry/v1/tarball"
)

func NewCmdPull() *cobra.Command {
	return &cobra.Command{
		Use:   "pull",
		Short: "Pull a remote image by reference and store its contents in a tarball",
		Args:  cobra.ExactArgs(2),
		Run:   pull,
	}
}

type ondiskCache struct {
	tmpdir string
}

func sanitizeHash(h v1.Hash) string {
	return strings.Replace(h.String(), ":", "_", -1)
}

func (c ondiskCache) Load(h v1.Hash) (io.ReadCloser, error) {
	f, err := os.Open(path.Join(c.tmpdir, sanitizeHash(h)))
	if os.IsNotExist(err) {
		return nil, remote.ErrCacheMiss
	} else if err != nil {
		return nil, err
	}
	log.Println("reading from cache", h)
	return f, nil
}

func (c ondiskCache) Store(h v1.Hash, rc io.Reader) error {
	f, err := os.Create(path.Join(c.tmpdir, sanitizeHash(h)))
	if err != nil {
		return err
	}
	_, err = io.Copy(f, rc)
	log.Println("wrote to cache", h)
	return err
}

func pull(_ *cobra.Command, args []string) {
	src, dst := args[0], args[1]
	// TODO: Why is only tag allowed?
	t, err := name.NewTag(src, name.WeakValidation)
	if err != nil {
		log.Fatalf("parsing tag %q: %v", src, err)
	}
	log.Printf("Pulling %v", t)

	auth, err := authn.DefaultKeychain.Resolve(t.Registry)
	if err != nil {
		log.Fatalf("getting creds for %q: %v", t, err)
	}

	i, err := remote.Image(t, auth, http.DefaultTransport, &remote.ImageOptions{
		Cache: ondiskCache{os.Getenv("HOME")},
	})
	if err != nil {
		log.Fatalf("reading image %q: %v", t, err)
	}

	if err := tarball.WriteToFile(dst, t, i, &tarball.WriteOptions{}); err != nil {
		log.Fatalf("writing image %q: %v", dst, err)
	}
}
