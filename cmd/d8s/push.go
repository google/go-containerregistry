package main

import (
	"context"
	"flag"
	"log"
	"net/http"

	"github.com/google/subcommands"

	"github.com/google/go-containerregistry/authn"
	"github.com/google/go-containerregistry/name"
	"github.com/google/go-containerregistry/v1/remote"
	"github.com/google/go-containerregistry/v1/tarball"
)

type pushCmd struct{}

func (*pushCmd) Name() string { return "push" }
func (*pushCmd) Synopsis() string {
	return "Pushes image contents as a tarball to a remote registry"
}
func (*pushCmd) Usage() string            { return "push <tarball> <image>" }
func (*pushCmd) SetFlags(f *flag.FlagSet) {}

func (*pushCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if len(f.Args()) != 2 {
		return subcommands.ExitUsageError
	}

	path, tag := f.Args()[0], f.Args()[1]
	t, err := name.NewTag(tag, name.WeakValidation)
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("Pulling %v", t)

	auth, err := authn.DefaultKeychain.Resolve(t.Registry)
	if err != nil {
		log.Fatalln(err)
	}

	i, err := remote.Image(t, auth, http.DefaultTransport)
	if err != nil {
		log.Fatalln(err)
	}

	if err := tarball.Write(path, t, i, &tarball.WriteOptions{}); err != nil {
		log.Fatalln(err)
	}
	return subcommands.ExitSuccess
}
