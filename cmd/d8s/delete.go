package main

import (
	"context"
	"flag"
	"log"
	"net/http"

	"github.com/google/go-containerregistry/authn"
	"github.com/google/go-containerregistry/name"
	"github.com/google/go-containerregistry/v1/remote"
	"github.com/google/subcommands"
)

type deleteCmd struct{}

func (*deleteCmd) Name() string             { return "delete" }
func (*deleteCmd) Synopsis() string         { return "Deletes a reference from its registry" }
func (*deleteCmd) Usage() string            { return "delete <image>" }
func (*deleteCmd) SetFlags(f *flag.FlagSet) {}

func (*deleteCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if len(f.Args()) != 1 {
		return subcommands.ExitUsageError
	}
	ref := f.Args()[0]

	r, err := name.ParseReference(ref, name.WeakValidation)
	if err != nil {
		log.Fatalln(err)
	}

	auth, err := authn.DefaultKeychain.Resolve(r.Context().Registry)
	if err != nil {
		log.Fatalln(err)
	}

	if err := remote.Delete(r, auth, http.DefaultTransport, remote.DeleteOptions{}); err != nil {
		log.Fatalln(err)
	}
	return subcommands.ExitSuccess
}
