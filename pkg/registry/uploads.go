// Copyright 2026 Google LLC All Rights Reserved.
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

package registry

import (
	"bytes"
	"io"
	"os"
)

const defaultUploadMemoryLimit = 32 << 20

type readCloser struct {
	io.Reader
	close func() error
}

func (rc *readCloser) Close() error {
	return rc.close()
}

type blobUpload struct {
	memoryLimit int64
	buf         bytes.Buffer
	file        *os.File
	size        int64
}

func (b *blobs) newUpload() *blobUpload {
	limit := b.uploadMemoryLimit
	if limit == 0 {
		limit = defaultUploadMemoryLimit
	}
	return newBlobUpload(limit)
}

func newBlobUpload(memoryLimit int64) *blobUpload {
	return &blobUpload{memoryLimit: memoryLimit}
}

func (u *blobUpload) Append(r io.Reader) error {
	if u.file != nil {
		n, err := io.Copy(u.file, r)
		u.size += n
		return err
	}

	remaining := u.memoryLimit - u.size
	if remaining >= 0 {
		n, err := io.CopyN(&u.buf, r, remaining+1)
		u.size += n
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}

	if err := u.spillToDisk(); err != nil {
		return err
	}
	n, err := io.Copy(u.file, r)
	u.size += n
	return err
}

func (u *blobUpload) Reader() (io.ReadCloser, error) {
	if u.file == nil {
		return io.NopCloser(bytes.NewReader(u.buf.Bytes())), nil
	}
	return io.NopCloser(io.NewSectionReader(u.file, 0, u.size)), nil
}

func (u *blobUpload) Size() int64 {
	return u.size
}

func (u *blobUpload) Cleanup() error {
	if u.file == nil {
		return nil
	}
	name := u.file.Name()
	if err := u.file.Close(); err != nil {
		return err
	}
	u.file = nil
	return os.Remove(name)
}

func (u *blobUpload) spillToDisk() error {
	f, err := os.CreateTemp("", "go-containerregistry-upload-*")
	if err != nil {
		return err
	}
	if _, err := f.Write(u.buf.Bytes()); err != nil {
		f.Close()
		os.Remove(f.Name())
		return err
	}
	u.buf.Reset()
	u.file = f
	return nil
}
