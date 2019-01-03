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

package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"sync"

	"github.com/mattmoor/dep-notify/pkg/graph"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/ko/build"
	"github.com/google/go-containerregistry/pkg/ko/publish"
	"github.com/google/go-containerregistry/pkg/ko/resolve"
	"github.com/google/go-containerregistry/pkg/name"
)

func gobuildOptions() ([]build.Option, error) {
	creationTime, err := getCreationTime()
	if err != nil {
		return nil, err
	}
	opts := []build.Option{
		build.WithBaseImages(getBaseImage),
	}
	if creationTime != nil {
		opts = append(opts, build.WithCreationTime(*creationTime))
	}
	return opts, nil
}

// resolvedFuture represents a "future" for the bytes of a resolved file.
type resolvedFuture chan []byte

func resolveFilesToWriter(fo *FilenameOptions, no *NameOptions, lo *LocalOptions, out io.WriteCloser) {
	defer out.Close()
	opt, err := gobuildOptions()
	if err != nil {
		log.Fatalf("error setting up builder options: %v", err)
	}
	builder, err := build.NewGo(opt...)
	if err != nil {
		log.Fatalf("error creating builder: %v", err)
	}

	// By having this as a channel, we can hook this up to a filesystem
	// watcher and leave `fs` open to stream the names of yaml files
	// affected by code changes (including the modification of existing or
	// creation of new yaml files).
	fs := enumerateFiles(fo)

	// This tracks filename -> []importpath
	var sm sync.Map

	var g graph.Interface
	var errCh chan error
	if fo.Watch {
		// Start a dep-notify process that on notifications scans the
		// file-to-recorded-build map and for each affected file resends
		// the filename along the channel.
		g, errCh, err = graph.New(func(ss graph.StringSet) {
			sm.Range(func(k, v interface{}) bool {
				key := k.(string)
				value := v.([]string)

				for _, ip := range value {
					if ss.Has(ip) {
						fs <- key
					}
				}
				return true
			})
		})
		if err != nil {
			log.Fatalf("Error creating dep-notify graph: %v", err)
		}
		// Cleanup the fsnotify hooks when we're done.
		defer g.Shutdown()
	}

	var futures []resolvedFuture
	for {
		// Each iteration, if there is anything in the list of futures,
		// listen to it in addition to the file enumerating channel.
		// A nil channel is never available to receive on, so if nothing
		// is available, this will result in us exclusively selecting
		// on the file enumerating channel.
		var bf resolvedFuture
		if len(futures) > 0 {
			bf = futures[0]
		} else if fs == nil {
			// There are no more files to enumerate and the futures
			// have been drained, so quit.
			break
		}

		select {
		case f, ok := <-fs:
			if !ok {
				// a nil channel is never available to receive on.
				// This allows us to drain the list of in-process
				// futures without this case of the select winning
				// each time.
				fs = nil
				break
			}

			// Make a new future to use to ship the bytes back and append
			// it to the list of futures (see comment below about ordering).
			ch := make(resolvedFuture)
			futures = append(futures, ch)

			// Kick off the resolution that will respond with its bytes on
			// the future.
			go func(f string) {
				defer close(ch)
				// Record the builds we do via this builder.
				recordingBuilder := &build.Recorder{
					Builder: builder,
				}
				b, err := resolveFile(f, no, lo, recordingBuilder)
				if err != nil {
					// Don't let build errors disrupt the watch.
					lg := log.Fatalf
					if fo.Watch {
						lg = log.Printf
					}
					lg("error processing import paths in %q: %v", f, err)
					return
				}
				// Associate with this file the collection of binary import paths.
				sm.Store(f, recordingBuilder.ImportPaths)
				ch <- b
				if fo.Watch {
					for _, ip := range recordingBuilder.ImportPaths {
						// Technically we never remove binary targets from the graph,
						// which will increase our graph's watch load, but the
						// notifications that they change will result in no affected
						// yamls, and no new builds or deploys.
						if err := g.Add(ip); err != nil {
							log.Fatalf("Error adding importpath to dep graph: %v", err)
						}
					}
				}
			}(f)

		case b, ok := <-bf:
			// Once the head channel returns something, dequeue it.
			// We listen to the futures in order to be respectful of
			// the kubectl apply ordering, which matters!
			futures = futures[1:]
			if ok {
				// Write the next body and a trailing delimiter.
				// We write the delimeter LAST so that when streamed to
				// kubectl it knows that the resource is complete and may
				// be applied.
				out.Write(append(b, []byte("\n---\n")...))
			}

		case err := <-errCh:
			log.Fatalf("Error watching dependencies: %v", err)
		}
	}
}

func resolveFile(f string, no *NameOptions, lo *LocalOptions, builder build.Interface) ([]byte, error) {
	var pub publish.Interface
	repoName := os.Getenv("KO_DOCKER_REPO")

	var namer publish.Namer
	if no.PreserveImportPaths {
		namer = preserveImportPath
	} else {
		namer = packageWithMD5
	}

	if lo.Local || repoName == publish.LocalDomain {
		pub = publish.NewDaemon(namer)
	} else {
		_, err := name.NewRepository(repoName, name.WeakValidation)
		if err != nil {
			return nil, fmt.Errorf("the environment variable KO_DOCKER_REPO must be set to a valid docker repository, got %v", err)
		}

		opts := []publish.Option{publish.WithAuthFromKeychain(authn.DefaultKeychain), publish.WithNamer(namer)}

		pub, err = publish.NewDefault(repoName, opts...)
		if err != nil {
			return nil, err
		}
	}

	var b []byte
	var err error
	if f == "-" {
		b, err = ioutil.ReadAll(os.Stdin)
	} else {
		b, err = ioutil.ReadFile(f)
	}
	if err != nil {
		return nil, err
	}

	return resolve.ImageReferences(b, builder, pub)
}
