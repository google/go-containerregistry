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
	"log"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

// From pkg/kubectl
type FilenameOptions struct {
	Filenames []string
	Recursive bool
	Watch     bool
}

func addFileArg(cmd *cobra.Command, fo *FilenameOptions) {
	// From pkg/kubectl
	cmd.Flags().StringSliceVarP(&fo.Filenames, "filename", "f", fo.Filenames,
		"Filename, directory, or URL to files to use to create the resource")
	cmd.Flags().BoolVarP(&fo.Recursive, "recursive", "R", fo.Recursive,
		"Process the directory used in -f, --filename recursively. Useful when you want to manage related manifests organized within the same directory.")
	cmd.Flags().BoolVarP(&fo.Watch, "watch", "W", fo.Watch,
		"Continuously monitor the transitive dependencies of the passed yaml files, and redeploy whenever anything changes.")
}

// Based heavily on pkg/kubectl
func enumerateFiles(fo *FilenameOptions) chan string {
	files := make(chan string)
	go func() {
		defer close(files)
		var watcher *fsnotify.Watcher
		if fo.Watch {
			var err error
			watcher, err = fsnotify.NewWatcher()
			if err != nil {
				log.Fatalf("Unexpected error initializing fsnotify: %v", err)
			}
			defer watcher.Close()
		}
		for _, paths := range fo.Filenames {
			if paths == "-" {
				files <- paths
				continue
			}
			err := filepath.Walk(paths, func(path string, fi os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				if fi.IsDir() {
					if path != paths && !fo.Recursive {
						return filepath.SkipDir
					}
					if watcher != nil {
						watcher.Add(path)
					}
					return nil
				}
				// Don't check extension if the filepath was passed explicitly
				if path != paths {
					switch filepath.Ext(path) {
					case ".json", ".yaml":
						// Process these.
					default:
						return nil
					}
				} else {
					if watcher != nil {
						watcher.Add(path)
					}
				}

				files <- path
				return nil
			})
			if err != nil {
				log.Fatalf("Error enumerating files: %v", err)
			}
		}

		if watcher != nil {
			for {
				select {
				case event := <-watcher.Events:
					switch filepath.Ext(event.Name) {
					case ".json", ".yaml":
						files <- event.Name
					}
				case err := <-watcher.Errors:
					log.Fatalf("Error watching: %v", err)
				}
			}
		}
	}()
	return files
}
