package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/google/subcommands"

	"github.com/google/go-containerregistry/authn"
	"github.com/google/go-containerregistry/name"
	"github.com/google/go-containerregistry/v1"
	"github.com/google/go-containerregistry/v1/remote"
)

func getImage(r string) (v1.Image, error) {
	ref, err := name.ParseReference(r, name.WeakValidation)
	if err != nil {
		return nil, err
	}
	auth, err := authn.DefaultKeychain.Resolve(ref.Context().Registry)
	if err != nil {
		return nil, err
	}
	return remote.Image(ref, auth, http.DefaultTransport)
}

type configCmd struct{}

func (*configCmd) Name() string             { return "config" }
func (*configCmd) Synopsis() string         { return "Prints the image's config" }
func (*configCmd) Usage() string            { return "config <image>" }
func (*configCmd) SetFlags(f *flag.FlagSet) {}

func (*configCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if len(f.Args()) != 1 {
		return subcommands.ExitUsageError
	}

	i, err := getImage(f.Args()[0])
	if err != nil {
		log.Fatalln(err)
	}
	config, err := i.RawConfigFile()
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(string(config))
	return subcommands.ExitSuccess
}

type digestCmd struct{}

func (*digestCmd) Name() string             { return "digest" }
func (*digestCmd) Synopsis() string         { return "Prints the image's digest" }
func (*digestCmd) Usage() string            { return "digest <image>" }
func (*digestCmd) SetFlags(f *flag.FlagSet) {}

func (*digestCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if len(f.Args()) != 1 {
		return subcommands.ExitUsageError
	}

	i, err := getImage(f.Args()[0])
	if err != nil {
		log.Fatalln(err)
	}
	digest, err := i.Digest()
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(digest.String())
	return subcommands.ExitSuccess
}

type manifestCmd struct{}

func (*manifestCmd) Name() string             { return "manifest" }
func (*manifestCmd) Synopsis() string         { return "Prints the image's manifest" }
func (*manifestCmd) Usage() string            { return "manifest <image>" }
func (*manifestCmd) SetFlags(f *flag.FlagSet) {}

func (*manifestCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if len(f.Args()) != 1 {
		return subcommands.ExitUsageError
	}

	i, err := getImage(f.Args()[0])
	if err != nil {
		log.Fatalln(err)
	}
	manifest, err := i.RawManifest()
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(string(manifest))
	return subcommands.ExitSuccess
}
