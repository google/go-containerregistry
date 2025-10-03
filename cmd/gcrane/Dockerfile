FROM gcr.io/distroless/static-debian12:nonroot

# Copy the built gcrane binary from the build context
COPY gcrane /usr/local/bin/gcrane

# Set the entrypoint
ENTRYPOINT ["/usr/local/bin/gcrane"]
