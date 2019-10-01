// Copyright 2019 Google LLC All Rights Reserved.
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

package validate

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

func Layer(layer v1.Layer) error {
	compressed, err := layer.Compressed()
	if err != nil {
		return err
	}

	// Keep track of compressed digest.
	digester := sha256.New()
	// Everything read from compressed is written to digester to compute digest.
	hashCompressed := io.TeeReader(compressed, digester)

	// Call io.Copy to write from the layer Reader through to the tarReader on
	// the other side of the pipe.
	pr, pw := io.Pipe()
	var size int64
	go func() {
		n, err := io.Copy(pw, hashCompressed)
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		size = n

		// Now close the compressed reader, to flush the gzip stream
		// and calculate digest/diffID/size. This will cause pr to
		// return EOF which will cause readers of the Compressed stream
		// to finish reading.
		pw.CloseWithError(compressed.Close())
	}()

	// Read the bytes through gzip.Reader to compute the DiffID.
	uncompressed, err := gzip.NewReader(pr)
	if err != nil {
		return err
	}
	diffider := sha256.New()
	hashUncompressed := io.TeeReader(uncompressed, diffider)

	// Ensure there aren't duplicate file paths.
	tarReader := tar.NewReader(hashUncompressed)
	files := make(map[string]struct{})
	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if _, ok := files[hdr.Name]; ok {
			return fmt.Errorf("duplicate file path: %s", hdr.Name)
		}
		files[hdr.Name] = struct{}{}
	}

	// Discard any trailing padding that the tar.Reader doesn't consume.
	if _, err := io.Copy(ioutil.Discard, hashUncompressed); err != nil {
		return err
	}

	if err := uncompressed.Close(); err != nil {
		return err
	}

	digest := v1.Hash{
		Algorithm: "sha256",
		Hex:       hex.EncodeToString(digester.Sum(make([]byte, 0, digester.Size()))),
	}

	diffid := v1.Hash{
		Algorithm: "sha256",
		Hex:       hex.EncodeToString(diffider.Sum(make([]byte, 0, diffider.Size()))),
	}

	ur, err := layer.Uncompressed()
	if err != nil {
		return err
	}
	udffid, _, err := v1.SHA256(ur)
	if err != nil {
		return err
	}
}
