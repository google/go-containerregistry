package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/google/go-containerregistry/pkg/registry"
)

var port = flag.Int("port", 1338, "port to run registry on")
var storage = flag.String("storage", "memory", "Option [memory, disk], default value is memory")
var diskLocation = flag.String("path", "", "Required when selecting disk storage. Path where the registry will store the blobs")
var splitRepositories = flag.Bool("split-repositories", false, "Separate blob storage per repository. This feature will ensures that each repository only has access to blobs inside the repository space.")

func main() {
	flag.Parse()

	var opts []registry.Option
	switch *storage {
	case "memory":
	case "disk":
		if *diskLocation == "" {
			log.Fatal("When storage is set to 'disk' a location has to be provided via the flag '--path'")
		}

		opts = append(opts, registry.DiskBlobStorage(*diskLocation))
	default:
		log.Fatalf("Option '%s' is not valid for storage flag. Choose 'memory' or 'disk'", *storage)
	}

	if *splitRepositories {
		opts = append(opts, registry.SplitBlobsByRepository())
	}

	s := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: registry.New(opts...),
	}
	log.Fatal(s.ListenAndServe())
}
