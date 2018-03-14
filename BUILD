load("@bazel_gazelle//:def.bzl", "gazelle")

licenses(["notice"])  # Apache 2.0

exports_files(["LICENSE"])

gazelle(
    name = "gazelle",
    command = "fix",
    external = "vendored",
    extra_args = [
        "-build_file_name",
        "BUILD.bazel,BUILD",  # Prioritize `BUILD.bazel` for newly added files.
    ],
    prefix = "github.com/google/go-containerregistry",
)
