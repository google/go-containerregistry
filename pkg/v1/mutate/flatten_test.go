// Copyright 2021 Google LLC All Rights Reserved.
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

package mutate

func TestFlatten(t *testing.T) {
	source, err := tarball.ImageFromPath("testdata/overwritten_file.tar", nil)
	if err != nil {
		t.Fatalf("Error loading image: %v", err)
	}
	result, err := mutate.Flatten(source)
	if err != nil {
		t.Fatalf("Unexpected error flattening image: %v", err)
	}

	layers := getLayers(t, result)

	if got, want := len(layers), 1; got != want {
		t.Fatalf("Layers did not return the flattened layer, got size %d want 1", len(layers))
	}

	sourceCf, err := source.ConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	cf, err := result.ConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	var want, got string
	want = cf.Architecture
	got = sourceCf.Architecture
	if want != got {
		t.Errorf("Incorrect architecture got=%q want=%q", got, want)
	}
	want = cf.OS
	got = sourceCf.OS
	if want != got {
		t.Errorf("Incorrect OS got=%q want=%q", got, want)
	}
	want = cf.OSVersion
	got = sourceCf.OSVersion
	if want != got {
		t.Errorf("Incorrect OSVersion got=%q want=%q", got, want)
	}

	if err := validate.Image(result); err != nil {
		t.Errorf("validate.Image() = %v", err)
	}
}