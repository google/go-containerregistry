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
	"bytes"
	"encoding/json"

	"github.com/google/go-containerregistry/v1"
	"github.com/google/go-containerregistry/v1/v1util"
)

// withConfigFile defines the subset of v1.Image used by these helper methods
// TODO(mattmoor): Consider renaming this to withCore?
type withConfigFile interface {
	imageCore
}

// DiffIDs is a helper for implementing v1.Image
func DiffIDs(i withConfigFile) ([]v1.Hash, error) {
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

// ConfigName is a helper for implementing v1.Image
func ConfigName(i withConfigFile) (v1.Hash, error) {
	config, err := i.ConfigFile()
	if err != nil {
		return v1.Hash{}, err
	}
	buf := bytes.NewBuffer(nil)
	if err := json.NewEncoder(buf).Encode(config); err != nil {
		return v1.Hash{}, err
	}
	h, _, err := v1.SHA256(v1util.NopReadCloser(buf))
	return h, err
}
