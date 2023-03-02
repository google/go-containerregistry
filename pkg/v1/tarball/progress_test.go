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

package tarball_test

import (
	"errors"
	"fmt"
	"io"
	"os"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

func ExampleWithProgress() {
	// buffered channel to make the example test easier
	c := make(chan v1.Update, 200)
	// Make a tempfile for tarball writes.
	fp, err := os.CreateTemp("", "")
	if err != nil {
		fmt.Printf("error creating temp file: %v\n", err)
		return
	}
	defer fp.Close()
	defer os.Remove(fp.Name())

	img, err := tarball.ImageFromPath("testdata/test_image_1.tar", nil)
	go func() {
		_ = tarball.WriteToFile(fp.Name(), nil, img, tarball.WithProgress(c))
	}()
	for update := range c {
		switch {
		case update.Error != nil && errors.Is(update.Error, io.EOF):
			fmt.Fprintf(os.Stderr, "receive error message: %v\n", err)
			fmt.Printf("%d/%d", update.Complete, update.Total)
			// Output: 4096/4096
			return
		case update.Error != nil:
			fmt.Printf("error writing tarball: %v\n", update.Error)
			return
		default:
			fmt.Fprintf(os.Stderr, "receive update: %#v\n", update)
		}
	}
}
