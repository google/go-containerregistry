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
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

func ExampleWithProgress() {
	/* calculations for this test:
	The image we are using is docker.io/library/alpine:3.10
	its size on disk is 2800640
	The filesizes inside are:
	-rw-r--r--  0 0      0        1509 Jan  1  1970 sha256:be4e4bea2c2e15b403bb321562e78ea84b501fb41497472e91ecb41504e8a27c
	-rw-r--r--  0 0      0     2795580 Jan  1  1970 21c83c5242199776c232920ddb58cfa2a46b17e42ed831ca9001c8dbc532d22d.tar.gz
	-rw-r--r--  0 0      0         216 Jan  1  1970 manifest.json
	when rounding each to a 512-byte block, plus the header, we get:
	1509    ->    1536 + 512 = 2048
	2795580 -> 2796032 + 512 = 2796544
	216     ->     512 + 512 = 1024
	add in 2 blocks of all 0x00 to indicate end of archive
	                         = 1024
	                        -------
	Total:                  2800640
	*/
	// buffered channel to make the example test easier
	c := make(chan v1.Update, 200)
	// Make a tempfile for tarball writes.
	fp, err := ioutil.TempFile("", "")
	if err != nil {
		fmt.Printf("error creating temp file: %v\n", err)
		return
	}
	defer fp.Close()
	defer os.Remove(fp.Name())

	tag, err := name.NewDigest("docker.io/library/alpine@sha256:f0e9534a598e501320957059cb2a23774b4d4072e37c7b2cf7e95b241f019e35", name.StrictValidation)
	if err != nil {
		fmt.Printf("error creating test tag: %v\n", err)
		return
	}
	desc, err := remote.Get(tag)
	if err != nil {
		fmt.Printf("error getting manifest: %v", err)
		return
	}
	img, err := desc.Image()
	if err != nil {
		fmt.Printf("error image: %v", err)
		return
	}
	go func() {
		_ = tarball.WriteToFile(fp.Name(), tag, img, tarball.WithProgress(c))
	}()
	for update := range c {
		switch {
		case update.Error != nil && update.Error == io.EOF:
			fmt.Fprintf(os.Stderr, "receive error message: %v\n", err)
			fmt.Printf("%d/%d", update.Complete, update.Total)
			// Output: 2800640/2800640
			return
		case update.Error != nil:
			fmt.Printf("error writing tarball: %v\n", update.Error)
			return
		default:
			fmt.Fprintf(os.Stderr, "receive update: %#v\n", update)
		}
	}
}
