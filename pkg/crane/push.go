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
	"fmt"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"sync"
)

// Load reads the tarball at path as a v1.Image.
func Load(path string, opt ...Option) (v1.Image, error) {
	return LoadTag(path, "")
}

// LoadMulti read the tarball from path as a map[name.Tag]v1.Image
func LoadMulti(path string, opt ...Option) (map[name.Reference]v1.Image, error) {
	o := makeOptions(opt...)
	return tarball.MultiImageFromPath(path, o.name...)
}

// LoadTag reads a tag from the tarball at path as a v1.Image.
// If tag is "", will attempt to read the tarball as a single image.
func LoadTag(path, tag string, opt ...Option) (v1.Image, error) {
	if tag == "" {
		return tarball.ImageFromPath(path, nil)
	}

	o := makeOptions(opt...)
	t, err := name.NewTag(tag, o.name...)
	if err != nil {
		return nil, fmt.Errorf("parsing tag %q: %v", tag, err)
	}
	return tarball.ImageFromPath(path, &t)
}

// Push pushes the v1.Image img to a registry as dst.
func Push(img v1.Image, dst string, opt ...Option) error {
	o := makeOptions(opt...)
	tag, err := name.ParseReference(dst, o.name...)
	if err != nil {
		return fmt.Errorf("parsing reference %q: %v", dst, err)
	}
	return remote.Write(tag, img, o.remote...)
}

// MultiPush pushes the map[name.Reference]v1.Image to a registry as dst.
func MultiPush(images map[name.Reference]v1.Image, dst string, concurrent int, opt ...Option) error {
	var wg sync.WaitGroup
	parallelsChan := make(chan bool, concurrent)
	resChan := make(chan pushResult)
	total := len(images)
	logs.Progress.Printf("pushing totally %d images\n", total)
	current := 1
	go func() {
		for res := range resChan {
			if res.err == nil {
				logs.Progress.Printf("INFO [%d/%d] push image %s/%s Successfully!\n", current, total, dst, res.image)
				current++
				continue
			}
			logs.Progress.Printf("push image [%s] to registry error: %s\n", res.image, res.err)
		}
	}()
	for t, i := range images {
		parallelsChan <- true
		wg.Add(1)
		var target string
		if tag, ok := t.(name.Tag); ok {
			target = fmt.Sprintf("%s/%s:%s", target, tag.RepositoryStr(), tag.TagStr())
		}
		if digest, ok := t.(name.Digest); ok {
			target = fmt.Sprintf("%s/%s:i-was-a-digest@%s", target, digest.RepositoryStr(), digest.DigestStr())
		}
		if target == "" {
			logs.Progress.Fatalf("error type for name.Reference %T", t)
		}
		go writeHelper(&wg, parallelsChan, resChan, target, i, opt...)
	}
	wg.Wait()
	return nil
}

type pushResult struct {
	err   error
	image string
}

func writeHelper(wg *sync.WaitGroup, concurrency chan bool, resCh chan pushResult, dst string, img v1.Image, options ...Option) {
	o := makeOptions(options...)
	tag, err := name.ParseReference(dst, o.name...)
	if err != nil {
		panic(err)
	}
	res := pushResult{image: dst}
	defer func() {
		resCh <- res
		wg.Done()
		<-concurrency
	}()
	res.err = remote.Write(tag, img, o.remote...)
}

// Upload pushes the v1.Layer to a given repo.
func Upload(layer v1.Layer, repo string, opt ...Option) error {
	o := makeOptions(opt...)
	ref, err := name.NewRepository(repo, o.name...)
	if err != nil {
		return fmt.Errorf("parsing repo %q: %v", repo, err)
	}

	return remote.WriteLayer(ref, layer, o.remote...)
}
