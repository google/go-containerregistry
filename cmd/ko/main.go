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
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/spf13/cobra"

	"github.com/google/go-containerregistry/authn"
	"github.com/google/go-containerregistry/ko/build"
	"github.com/google/go-containerregistry/ko/publish"
	"github.com/google/go-containerregistry/ko/resolve"
	"github.com/google/go-containerregistry/name"
	"github.com/google/go-containerregistry/v1/remote"
)

var (
	baseImage, _ = name.NewTag("gcr.io/distroless/base:latest", name.WeakValidation)
)

// runCmd is suitable for use with cobra.Command's Run field.
type runCmd func(*cobra.Command, []string)

// passthru returns a runCmd that simply passes our CLI arguments
// through to a binary named command.
func passthru(command string) runCmd {
	return func(_ *cobra.Command, _ []string) {
		// Start building a command line invocation by passing
		// through our arguments to command's CLI.
		cmd := exec.Command(command, os.Args[1:]...)

		// Pass through our environment
		cmd.Env = os.Environ()
		// Pass through our stdfoo
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		cmd.Stdin = os.Stdin

		// Run it.
		if err := cmd.Run(); err != nil {
			log.Fatalln(err)
		}
	}
}

// addGoCommands augments our CLI surface with passthru sub-commands for the Go CLI surface.
func addGoCommands(topLevel *cobra.Command) {
	// For convenience, we expose top-level commands for each of the top-level "go foo" commands.
	// TODO(mattmoor): Split out some of these where we can splice in additional goodness.
	// e.g. could we make `ko fmt` format K8s yamls?
	foos := []string{"build", "clean", "doc", "env", "bug", "fix", "fmt", "generate", "get",
		"install", "list", "run", "test", "tool", "version", "vet"}
	for _, foo := range foos {
		topLevel.AddCommand(&cobra.Command{
			Use:   foo,
			Short: fmt.Sprintf(`See "go help %s" for detailed usage.`, foo),
			Run:   passthru("go"),
			// We ignore unknown flags to avoid importing everything Go exposes
			// from our commands.
			FParseErrWhitelist: cobra.FParseErrWhitelist{
				UnknownFlags: true,
			},
		})
	}
}

// addKubeCommands augments our CLI surface with a passthru delete command, and an apply
// command that realizes the promise of ko, as outlined here:
//    https://github.com/google/go-containerregistry/issues/80
func addKubeCommands(topLevel *cobra.Command) {
	topLevel.AddCommand(&cobra.Command{
		Use:   "delete",
		Short: `See "kubectl help delete" for detailed usage.`,
		Run:   passthru("kubectl"),
		// We ignore unknown flags to avoid importing everything Go exposes
		// from our commands.
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
	})
	fo := &FilenameOptions{}
	apply := &cobra.Command{
		Use: "apply -f FILENAME",
		// TODO(mattmoor): Expose our own apply surface.
		Short: `See "kubectl help apply" for detailed usage.`,
		Run: func(cmd *cobra.Command, args []string) {
			// TODO(mattmoor): Use io.Pipe to avoid buffering the whole thing.
			buf := bytes.NewBuffer(nil)
			resolveFilesTo(fo, buf)

			// Issue a "kubectl apply" command reading from stdin,
			// to which we will pipe the resolved files.
			kubectlCmd := exec.Command("kubectl", "apply", "-f", "-")

			// Pass through our environment
			kubectlCmd.Env = os.Environ()
			// Pass through our std{out,err} and make our resolved buffer stdin.
			kubectlCmd.Stderr = os.Stderr
			kubectlCmd.Stdout = os.Stdout
			kubectlCmd.Stdin = buf

			// Run it.
			if err := kubectlCmd.Run(); err != nil {
				log.Fatalln(err)
			}
		},
	}
	addFileArg(apply, fo)
	topLevel.AddCommand(apply)
	resolve := &cobra.Command{
		// TODO(mattmoor): Pick a better name.
		Use:   "resolve -f FILENAME",
		Short: `Print the input files with image references resolved to built/pushed image digests.`,
		Run: func(cmd *cobra.Command, args []string) {
			resolveFilesTo(fo, os.Stdout)
		},
	}
	addFileArg(resolve, fo)
	topLevel.AddCommand(resolve)
}

func gobuildOptions() build.Options {
	// A better story for O
	base, err := remote.Image(baseImage, authn.Anonymous, http.DefaultTransport)
	if err != nil {
		log.Fatalln(err)
	}
	return build.Options{
		Base: base,
	}
}

// From pkg/kubectl
type FilenameOptions struct {
	Filenames []string
	Recursive bool
}

func addFileArg(cmd *cobra.Command, fo *FilenameOptions) {
	// From pkg/kubectl
	cmd.Flags().StringSliceVarP(&fo.Filenames, "filename", "f", fo.Filenames,
		"Filename, directory, or URL to files to use to create the resource")
	cmd.Flags().BoolVarP(&fo.Recursive, "recursive", "R", fo.Recursive,
		"Process the directory used in -f, --filename recursively. Useful when you want to manage related manifests organized within the same directory.")
}

// Based heavily on pkg/kubectl
func enumerateFiles(fo *FilenameOptions) ([]string, error) {
	var files []string
	for _, paths := range fo.Filenames {
		err := filepath.Walk(paths, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if fi.IsDir() {
				if path != paths && !fo.Recursive {
					return filepath.SkipDir
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
			}

			files = append(files, path)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

func resolveFilesTo(fo *FilenameOptions, out io.Writer) {
	fs, err := enumerateFiles(fo)
	if err != nil {
		log.Fatalln(err)
	}

	opt := gobuildOptions()
	var sm sync.Map
	wg := sync.WaitGroup{}
	for _, f := range fs {
		wg.Add(1)
		go func(f string) {
			defer wg.Done()

			b, err := resolveFile(f, opt)
			if err != nil {
				log.Fatalln(err)
			}
			sm.Store(f, b)
		}(f)
	}
	// Wait for all of them to complete.
	wg.Wait()
	for _, f := range fs {
		iface, ok := sm.Load(f)
		if !ok {
			log.Fatalf("missing file in resolved map: %v", f)
		}
		b, ok := iface.([]byte)
		if !ok {
			log.Fatalf("unsupported type in sync.Map's value: %T", iface)
		}
		// Our sole output should be the resolved yamls
		out.Write([]byte("---\n"))
		out.Write(b)
	}
}

func resolveFile(f string, opt build.Options) ([]byte, error) {
	repoName := os.Getenv("KO_DOCKER_REPO")
	repo, err := name.NewRepository(repoName, name.WeakValidation)
	if err != nil {
		return nil, fmt.Errorf("the environment variable KO_DOCKER_REPO must be set to a valid docker repository, got %v", err)
	}

	b, err := ioutil.ReadFile(f)
	if err != nil {
		return nil, err
	}

	publisher := publish.NewDefault(repo, http.DefaultTransport, remote.WriteOptions{
		MountPaths: []name.Repository{baseImage.Repository},
	})
	builder, err := build.NewGo(opt)
	if err != nil {
		return nil, err
	}

	// TODO(mattmoor): To better approximate Bazel, we should collect the importpath references
	// in advance, trigger builds, and then do a second pass to finalize each of the configs.
	b2, err := resolve.ImageReferences(b, builder, publisher)
	if err != nil {
		return nil, err
	}
	return b2, err
}

func main() {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:   "ko",
		Short: "Rapidly iterate with Go, Containers, and Kubernetes.",
		Long:  "Long Desc", // K8s has a helper here?
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
	addGoCommands(cmds)
	addKubeCommands(cmds)

	if err := cmds.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
