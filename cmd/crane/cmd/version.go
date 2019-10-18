package cmd

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// Version can be set via:
// -ldflags="-X 'github.com/google/go-containerregistry/pkg/crane.Version=$TAG'"
var Version string

func init() { Root.AddCommand(NewCmdVersion()) }

// NewCmdVersion creates a new cobra.Command for the version subcommand.
func NewCmdVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			if Version == "" {
				i, ok := debug.ReadBuildInfo()
				if !ok {
					fmt.Println("could not determine build information")
					return
				}
				Version = i.Main.Version
			}
			fmt.Println(Version)
		},
	}
}
