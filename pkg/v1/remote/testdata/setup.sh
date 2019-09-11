#!/bin/sh

docker build -t gocontainerregistry/testing:latest .
docker save gocontainerregistry/testing:latest > testimage.tar
