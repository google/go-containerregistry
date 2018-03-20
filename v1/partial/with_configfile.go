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

package partial

import (
	"github.com/google/go-containerregistry/v1"
)

// WithConfigFile defines the subset of v1.Image used by these helper methods
// TODO(mattmoor): Consider renaming this to WithCore?
type WithConfigFile interface {
	imageCore
}

// DiffIDs is a helper for implementing v1.Image
func DiffIDs(i WithConfigFile) ([]v1.Hash, error) {
	cfg, err := i.ConfigFile()
	if err != nil {
		return nil, err
	}
	dids := make([]v1.Hash, len(cfg.RootFS.DiffIDs))
	for i, did := range cfg.RootFS.DiffIDs {
		dids[len(dids)-i-1] = did
	}
	return dids, nil
}
