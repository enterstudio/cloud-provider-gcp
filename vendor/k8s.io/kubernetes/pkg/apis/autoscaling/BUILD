package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = [
        "annotations.go",
        "doc.go",
        "register.go",
        "types.go",
        "zz_generated.deepcopy.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/apis/autoscaling",
    importpath = "k8s.io/kubernetes/pkg/apis/autoscaling",
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/api/resource:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/core:go_default_library",
    ],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [
        ":package-srcs",
        "//pkg/apis/autoscaling/fuzzer:all-srcs",
        "//pkg/apis/autoscaling/install:all-srcs",
        "//pkg/apis/autoscaling/v1:all-srcs",
        "//pkg/apis/autoscaling/v2beta1:all-srcs",
        "//pkg/apis/autoscaling/validation:all-srcs",
    ],
    tags = ["automanaged"],
)
