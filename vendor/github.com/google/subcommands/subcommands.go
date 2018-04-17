/*
Copyright 2016 Google Inc. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package subcommands implements a simple way for a single command to have many
// subcommands, each of which takes arguments and so forth.
package subcommands

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
)

// A Command represents a single command.
type Command interface {
	// Name returns the name of the command.
	Name() string

	// Synopsis returns a short string (less than one line) describing the command.
	Synopsis() string

	// Usage returns a long string explaining the command and giving usage
	// information.
	Usage() string

	// SetFlags adds the flags for this command to the specified set.
	SetFlags(*flag.FlagSet)

	// Execute executes the command and returns an ExitStatus.
	Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) ExitStatus
}

// A Commander represents a set of commands.
type Commander struct {
	commands  []*commandGroup
	topFlags  *flag.FlagSet // top-level flags
	important []string      // important top-level flags
	name      string        // normally path.Base(os.Args[0])

	Output io.Writer // Output specifies where the commander should write its output (default: os.Stdout).
	Error  io.Writer // Error specifies where the commander should write its error (default: os.Stderr).
}

// A commandGroup represents a set of commands about a common topic.
type commandGroup struct {
	name     string
	commands []Command
}

// An ExitStatus represents a Posix exit status that a subcommand
// expects to be returned to the shell.
type ExitStatus int

const (
	ExitSuccess ExitStatus = iota
	ExitFailure
	ExitUsageError
)

// NewCommander returns a new commander with the specified top-level
// flags and command name. The Usage function for the topLevelFlags
// will be set as well.
func NewCommander(topLevelFlags *flag.FlagSet, name string) *Commander {
	cdr := &Commander{
		topFlags: topLevelFlags,
		name:     name,
		Output:   os.Stdout,
		Error:    os.Stderr,
	}
	topLevelFlags.Usage = func() { cdr.explain(cdr.Error) }
	return cdr
}

// Register adds a subcommand to the supported subcommands in the
// specified group. (Help output is sorted and arranged by group name.)
// The empty string is an acceptable group name; such subcommands are
// explained first before named groups.
func (cdr *Commander) Register(cmd Command, group string) {
	for _, g := range cdr.commands {
		if g.name == group {
			g.commands = append(g.commands, cmd)
			return
		}
	}
	cdr.commands = append(cdr.commands, &commandGroup{
		name:     group,
		commands: []Command{cmd},
	})
}

// ImportantFlag marks a top-level flag as important, which means it
// will be printed out as part of the output of an ordinary "help"
// subcommand.  (All flags, important or not, are printed by the
// "flags" subcommand.)
func (cdr *Commander) ImportantFlag(name string) {
	cdr.important = append(cdr.important, name)
}

// Execute should be called once the top-level-flags on a Commander
// have been initialized. It finds the correct subcommand and executes
// it, and returns an ExitStatus with the result. On a usage error, an
// appropriate message is printed to os.Stderr, and ExitUsageError is
// returned. The additional args are provided as-is to the Execute method
// of the selected Command.
func (cdr *Commander) Execute(ctx context.Context, args ...interface{}) ExitStatus {
	if cdr.topFlags.NArg() < 1 {
		cdr.topFlags.Usage()
		return ExitUsageError
	}

	name := cdr.topFlags.Arg(0)

	for _, group := range cdr.commands {
		for _, cmd := range group.commands {
			if name != cmd.Name() {
				continue
			}
			f := flag.NewFlagSet(name, flag.ContinueOnError)
			f.Usage = func() { explain(cdr.Error, cmd) }
			cmd.SetFlags(f)
			if f.Parse(cdr.topFlags.Args()[1:]) != nil {
				return ExitUsageError
			}
			return cmd.Execute(ctx, f, args...)
		}
	}

	// Cannot find this command.
	cdr.topFlags.Usage()
	return ExitUsageError
}

// Sorting of a slice of command groups.
type byGroupName []*commandGroup

func (p byGroupName) Len() int           { return len(p) }
func (p byGroupName) Less(i, j int) bool { return p[i].name < p[j].name }
func (p byGroupName) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// explain prints a brief description of all the subcommands and the
// important top-level flags.
func (cdr *Commander) explain(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s <flags> <subcommand> <subcommand args>\n\n", cdr.name)
	sort.Sort(byGroupName(cdr.commands))
	for _, group := range cdr.commands {
		explainGroup(w, group)
	}
	if cdr.topFlags == nil {
		fmt.Fprintln(w, "\nNo top level flags.")
		return
	}
	if len(cdr.important) == 0 {
		fmt.Fprintf(w, "\nUse \"%s flags\" for a list of top-level flags\n", cdr.name)
		return
	}

	fmt.Fprintf(w, "\nTop-level flags (use \"%s flags\" for a full list):\n", cdr.name)
	for _, name := range cdr.important {
		f := cdr.topFlags.Lookup(name)
		if f == nil {
			panic(fmt.Sprintf("Important flag (%s) is not defined", name))
		}
		fmt.Fprintf(w, "  -%s=%s: %s\n", f.Name, f.DefValue, f.Usage)
	}
}

// Sorting of the commands within a group.
func (g commandGroup) Len() int           { return len(g.commands) }
func (g commandGroup) Less(i, j int) bool { return g.commands[i].Name() < g.commands[j].Name() }
func (g commandGroup) Swap(i, j int)      { g.commands[i], g.commands[j] = g.commands[j], g.commands[i] }

// explainGroup explains all the subcommands for a particular group.
func explainGroup(w io.Writer, group *commandGroup) {
	if len(group.commands) == 0 {
		return
	}
	if group.name == "" {
		fmt.Fprintf(w, "Subcommands:\n")
	} else {
		fmt.Fprintf(w, "Subcommands for %s:\n", group.name)
	}
	sort.Sort(group)
	for _, cmd := range group.commands {
		fmt.Fprintf(w, "\t%-15s  %s\n", cmd.Name(), cmd.Synopsis())
	}
	fmt.Fprintln(w)
}

// explainCmd prints a brief description of a single command.
func explain(w io.Writer, cmd Command) {
	fmt.Fprintf(w, "%s", cmd.Usage())
	subflags := flag.NewFlagSet(cmd.Name(), flag.PanicOnError)
	subflags.SetOutput(w)
	cmd.SetFlags(subflags)
	subflags.PrintDefaults()
}

// A helper is a Command implementing a "help" command for
// a given Commander.
type helper Commander

func (h *helper) Name() string           { return "help" }
func (h *helper) Synopsis() string       { return "describe subcommands and their syntax" }
func (h *helper) SetFlags(*flag.FlagSet) {}
func (h *helper) Usage() string {
	return `help [<subcommand>]:
	With an argument, prints detailed information on the use of
	the specified subcommand. With no argument, print a list of
	all commands and a brief description of each.
`
}
func (h *helper) Execute(_ context.Context, f *flag.FlagSet, args ...interface{}) ExitStatus {
	switch f.NArg() {
	case 0:
		(*Commander)(h).explain(h.Output)
		return ExitSuccess

	case 1:
		for _, group := range h.commands {
			for _, cmd := range group.commands {
				if f.Arg(0) != cmd.Name() {
					continue
				}
				explain(h.Output, cmd)
				return ExitSuccess
			}
		}
		fmt.Fprintf(h.Error, "Subcommand %s not understood\n", f.Arg(0))
	}

	f.Usage()
	return ExitUsageError
}

// HelpCommand returns a Command which implements a "help" subcommand.
func (cdr *Commander) HelpCommand() Command {
	return (*helper)(cdr)
}

// A flagger is a Command implementing a "flags" command for a given Commander.
type flagger Commander

func (flg *flagger) Name() string           { return "flags" }
func (flg *flagger) Synopsis() string       { return "describe all known top-level flags" }
func (flg *flagger) SetFlags(*flag.FlagSet) {}
func (flg *flagger) Usage() string {
	return `flags [<subcommand>]:
	With an argument, print all flags of <subcommand>. Else,
	print a description of all known top-level flags.  (The basic
	help information only discusses the most generally important
	top-level flags.)
`
}
func (flg *flagger) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) ExitStatus {
	if f.NArg() > 1 {
		f.Usage()
		return ExitUsageError
	}

	if f.NArg() == 0 {
		if flg.topFlags == nil {
			fmt.Fprintln(flg.Output, "No top-level flags are defined.")
		} else {
			flg.topFlags.PrintDefaults()
		}
		return ExitSuccess
	}

	for _, group := range flg.commands {
		for _, cmd := range group.commands {
			if f.Arg(0) != cmd.Name() {
				continue
			}
			subflags := flag.NewFlagSet(cmd.Name(), flag.PanicOnError)
			subflags.SetOutput(flg.Output)
			cmd.SetFlags(subflags)
			subflags.PrintDefaults()
			return ExitSuccess
		}
	}
	fmt.Fprintf(flg.Error, "Subcommand %s not understood\n", f.Arg(0))
	return ExitFailure
}

// FlagsCommand returns a Command which implements a "flags" subcommand.
func (cdr *Commander) FlagsCommand() Command {
	return (*flagger)(cdr)
}

// A lister is a Command implementing a "commands" command for a given Commander.
type lister Commander

func (l *lister) Name() string           { return "commands" }
func (l *lister) Synopsis() string       { return "list all command names" }
func (l *lister) SetFlags(*flag.FlagSet) {}
func (l *lister) Usage() string {
	return `commands:
	Print a list of all commands.
`
}
func (l *lister) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) ExitStatus {
	if f.NArg() != 0 {
		f.Usage()
		return ExitUsageError
	}

	for _, group := range l.commands {
		for _, cmd := range group.commands {
			fmt.Fprintf(l.Output, "%s\n", cmd.Name())
		}
	}
	return ExitSuccess
}

// CommandsCommand returns Command which implements a "commands" subcommand.
func (cdr *Commander) CommandsCommand() Command {
	return (*lister)(cdr)
}

// DefaultCommander is the default commander using flag.CommandLine for flags
// and os.Args[0] for the command name.
var DefaultCommander *Commander

func init() {
	DefaultCommander = NewCommander(flag.CommandLine, path.Base(os.Args[0]))
}

// Register adds a subcommand to the supported subcommands in the
// specified group. (Help output is sorted and arranged by group
// name.)  The empty string is an acceptable group name; such
// subcommands are explained first before named groups. It is a
// wrapper around DefaultCommander.Register.
func Register(cmd Command, group string) {
	DefaultCommander.Register(cmd, group)
}

// ImportantFlag marks a top-level flag as important, which means it
// will be printed out as part of the output of an ordinary "help"
// subcommand.  (All flags, important or not, are printed by the
// "flags" subcommand.) It is a wrapper around
// DefaultCommander.ImportantFlag.
func ImportantFlag(name string) {
	DefaultCommander.ImportantFlag(name)
}

// Execute should be called once the default flags have been
// initialized by flag.Parse. It finds the correct subcommand and
// executes it, and returns an ExitStatus with the result. On a usage
// error, an appropriate message is printed to os.Stderr, and
// ExitUsageError is returned. The additional args are provided as-is
// to the Execute method of the selected Command. It is a wrapper
// around DefaultCommander.Execute.
func Execute(ctx context.Context, args ...interface{}) ExitStatus {
	return DefaultCommander.Execute(ctx, args...)
}

// HelpCommand returns a Command which implements "help" for the
// DefaultCommander. Use Register(HelpCommand(), <group>) for it to be
// recognized.
func HelpCommand() Command {
	return DefaultCommander.HelpCommand()
}

// FlagsCommand returns a Command which implements "flags" for the
// DefaultCommander. Use Register(FlagsCommand(), <group>) for it to be
// recognized.
func FlagsCommand() Command {
	return DefaultCommander.FlagsCommand()
}

// CommandsCommand returns Command which implements a "commands" subcommand.
func CommandsCommand() Command {
	return DefaultCommander.CommandsCommand()
}
