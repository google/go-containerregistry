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

package gcrane

import (
	"os"

	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/spf13/cobra"
)

func init() {
	Root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable debug logs")
}

var (
	verbose = false

	// Root is the top-level cobra.Command for gcrane.
	Root = &cobra.Command{
		Use:               "gcrane",
		Short:             "gcrane is a tool for managing container images on gcr.io",
		Run:               func(cmd *cobra.Command, _ []string) { cmd.Usage() },
		DisableAutoGenTag: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if verbose {
				logs.Debug.SetOutput(os.Stderr)
			}
		},
	}
)
