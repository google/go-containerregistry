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
	"net/http"

	"github.com/google/go-containerregistry/pkg/registry"
)

var port = flag.Int("port", 1338, "port to run registry on")

func main() {
	flag.Parse()
	s := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: registry.New(),
	}
	log.Fatal(s.ListenAndServe())
}
