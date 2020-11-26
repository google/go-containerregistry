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

package remote

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"golang.org/x/sync/errgroup"
)

// MultiDelete removes the specified image references from remote registries.
//
// Current limitations:
// - All refs must share the same repository.
func MultiDelete(refs []name.Reference, options ...Option) error {
	// Determine the repository being pushed to; if asked to push to
	// multiple repositories, give up.
	var repo, zero name.Repository
	for _, ref := range refs {
		if repo == zero {
			repo = ref.Context()
		} else if ref.Context() != repo {
			return fmt.Errorf("MultiWrite can only push to the same repository (saw %q and %q)", repo, ref.Context())
		}
	}

	o, err := makeOptions(repo, options...)
	if err != nil {
		return err
	}
	scopes := []string{repo.Scope(transport.DeleteScope)}
	tr, err := transport.NewWithContext(o.context, repo.Registry, o.auth, o.transport, scopes)
	if err != nil {
		return err
	}
	c := &http.Client{Transport: tr}

	ch := make(chan name.Reference, 2*o.jobs)
	var g errgroup.Group
	for i := 0; i < o.jobs; i++ {
		// Start N workers consuming refs to delete.
		g.Go(func() error {
			for ref := range ch {
				if err := deleteOne(ref, c, o); err != nil {
					return err
				}
			}
			return nil
		})
	}
	go func() {
		for _, ref := range refs {
			ch <- ref
		}
		close(ch)
	}()
	return g.Wait()
}

func deleteOne(ref name.Reference, c *http.Client, o *options) error {
	u := url.URL{
		Scheme: ref.Context().Registry.Scheme(),
		Host:   ref.Context().RegistryStr(),
		Path:   fmt.Sprintf("/v2/%s/manifests/%s", ref.Context().RepositoryStr(), ref.Identifier()),
	}
	req, err := http.NewRequest(http.MethodDelete, u.String(), nil)
	if err != nil {
		return err
	}

	resp, err := c.Do(req.WithContext(o.context))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return transport.CheckError(resp, http.StatusOK, http.StatusAccepted)
}

// Delete removes the specified image reference from the remote registry.
func Delete(ref name.Reference, options ...Option) error {
	return MultiDelete([]name.Reference{ref}, options...)
}
