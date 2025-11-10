"""Aspect to collect package_info metadata for buildInfo.

This aspect traverses Go binary dependencies and collects version information
from Gazelle-generated package_info targets referenced via the package_metadata
common attribute (inherited from REPO.bazel default_package_metadata).

Implementation based on bazel-contrib/supply-chain gather_metadata pattern.
Currently doesn't use the supply chain tools dep for this as it is not yet
stable and we still need to support WORKSPACE which the supply-chain tools
doesn't have support for.
"""

load(
    "//go/private:providers.bzl",
    "GoArchive",
)

visibility(["//go/private/..."])

BuildInfoMetadata = provider(
    doc = "INTERNAL: Provides dependency version metadata for buildInfo. Do not depend on this provider.",
    fields = {
        "importpaths": "Depset of importpath strings from Go dependencies",
        "metadata_providers": "Depset of PackageInfo providers with version data",
    },
)

def _buildinfo_aspect_impl(target, ctx):
    """Collects package_info metadata from dependencies.

    This aspect collects version information from package_metadata attributes
    (set via REPO.bazel default_package_metadata in go_repository) and tracks
    importpaths from Go dependencies. The actual version matching is deferred
    to execution time to avoid quadratic memory usage.

    Args:
        target: The target being visited
        ctx: The aspect context

    Returns:
        List containing BuildInfoMetadata provider
    """
    direct_importpaths = []
    direct_metadata = []
    transitive_importpaths = []
    transitive_metadata = []

    # Collect importpath from this target if it's a Go target
    if GoArchive in target:
        importpath = target[GoArchive].data.importpath
        if importpath:
            direct_importpaths.append(importpath)

    # Check package_metadata common attribute (Bazel 6+)
    # This is set via REPO.bazel default_package_metadata in go_repository
    if hasattr(ctx.rule.attr, "package_metadata"):
        attr_value = ctx.rule.attr.package_metadata
        if attr_value:
            package_metadata = attr_value if type(attr_value) == type([]) else [attr_value]
            # Store the metadata targets directly for later processing
            direct_metadata.extend(package_metadata)

    # Collect transitive metadata from Go dependencies only
    # Only traverse deps and embed to avoid non-Go dependencies
    for attr_name in ["deps", "embed"]:
        if not hasattr(ctx.rule.attr, attr_name):
            continue

        attr_value = getattr(ctx.rule.attr, attr_name)
        if not attr_value:
            continue

        deps = attr_value if type(attr_value) == type([]) else [attr_value]
        for dep in deps:
            # Collect transitive BuildInfoMetadata
            if BuildInfoMetadata in dep:
                transitive_importpaths.append(dep[BuildInfoMetadata].importpaths)
                transitive_metadata.append(dep[BuildInfoMetadata].metadata_providers)

    # Build depsets (empty depsets are efficient, no need for early return)
    return [BuildInfoMetadata(
        importpaths = depset(
            direct = direct_importpaths,
            transitive = transitive_importpaths,
        ),
        metadata_providers = depset(
            direct = direct_metadata,
            transitive = transitive_metadata,
        ),
    )]

buildinfo_aspect = aspect(
    doc = "Collects package_info metadata for Go buildInfo",
    implementation = _buildinfo_aspect_impl,
    attr_aspects = ["deps", "embed"],  # Only traverse Go dependencies
    provides = [BuildInfoMetadata],
    # Apply to generated targets including package_info
    apply_to_generating_rules = True,
)
