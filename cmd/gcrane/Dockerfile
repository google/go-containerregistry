# Use a minimal base image from an approved source
FROM us-docker.pkg.dev/gke-release/distroless/static:nonroot

# Copy the built gcrane binary from the build context
COPY gcrane /usr/local/bin/gcrane

# Set the entrypoint
ENTRYPOINT ["/usr/local/bin/gcrane"]
