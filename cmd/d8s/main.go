package main

import (
	"context"
	"flag"
	"os"

	"github.com/google/subcommands"
)

func main() {
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")
	subcommands.Register(&configCmd{}, "")
	subcommands.Register(&deleteCmd{}, "")
	subcommands.Register(&digestCmd{}, "")
	subcommands.Register(&manifestCmd{}, "")
	subcommands.Register(&pullCmd{}, "")
	subcommands.Register(&pushCmd{}, "")

	flag.Parse()
	ctx := context.Background()
	os.Exit(int(subcommands.Execute(ctx)))
}
