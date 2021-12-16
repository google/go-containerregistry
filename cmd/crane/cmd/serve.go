// Copyright 2021 Google LLC All Rights Reserved.
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

package cmd

import (
	"fmt"
	"log"
	"net/http"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/spf13/cobra"
)

// NewCmdServe creates a new cobra.Command to start a registry server.
func NewCmdServe(options *[]crane.Option) *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start a local registry server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s := &http.Server{
				Addr:    fmt.Sprintf(":%d", port),
				Handler: registry.New(),
			}
			go func() {
				<-ctx.Done()
				s.Shutdown(ctx)
			}()
			log.Printf("Listening on port %d", port)
			return s.ListenAndServe()
		},
	}

	cmd.Flags().IntVar(&port, "port", 1338, "Port to listen on")
	return cmd
}
