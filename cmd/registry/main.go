// Copyright 2019 Google LLC All Rights Reserved.
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
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/google/go-containerregistry/pkg/registry"
)

var port = flag.Int("port", 1338, "port to run registry on")

func main() {
	flag.Parse()

	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", *port))
	if err != nil {
		log.Fatal(err)
	}
	porti := listener.Addr().(*net.TCPAddr).Port
	log.Printf("serving on port %d", porti)
	s := &http.Server{
		ReadHeaderTimeout: 5 * time.Second, // prevent slowloris, quiet linter
		Handler: registry.New(
			registry.WithWarning(.01, "Congratulations! You've won a lifetime's supply of free image pulls from this in-memory registry!"),
		),
	}
	log.Fatal(s.Serve(listener))
}
