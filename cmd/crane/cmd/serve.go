// Copyright 2023 Google LLC All Rights Reserved.
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
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/phayes/freeport"
	"github.com/spf13/cobra"

	"github.com/google/go-containerregistry/pkg/registry"
)

func NewCmdServe() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Serve an in-memory registry implementation",
		Long: `This sub-command serves an in-memory registry implementation on port :8080 (or $PORT)

The command blocks while the server accepts pushes and pulls.

Contents are only stored in memory, and when the process exits, pushed data is lost.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			port := os.Getenv("PORT")
			if port == "" {
				porti, err := freeport.GetFreePort()
				if err != nil {
					return err
				}
				port = fmt.Sprintf("%d", porti)
			}

			s := &http.Server{
				Addr:              fmt.Sprintf(":%s", port),
				ReadHeaderTimeout: 5 * time.Second, // prevent slowloris, quiet linter
				Handler:           registry.New(),
			}
			log.Printf("serving on port %s", port)

			errCh := make(chan error)
			go func() { errCh <- s.ListenAndServe() }()

			<-ctx.Done()
			log.Println("shutting down...")
			if err := s.Shutdown(ctx); err != nil {
				return err
			}

			if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		},
	}
}
