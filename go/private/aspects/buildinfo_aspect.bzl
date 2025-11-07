"""Aspect to collect package_info metadata for buildInfo.

This aspect traverses Go binary dependencies and collects version information
from Gazelle-generated package_info targets referenced via the package_metadata
common attribute (inherited from REPO.bazel default_package_metadata).

Implementation based on bazel-contrib/supply-chain gather_metadata pattern.
"""

load(
    "//go/private:providers.bzl",
    "GoArchive",
)

BuildInfoMetadata = provider(
    doc = "Provides dependency version metadata for buildInfo",
    fields = {
        "version_map": "Depset of (importpath, version) tuples",
    },
)

def _buildinfo_aspect_impl(target, ctx):
    """Collects package_info metadata from dependencies.

    Following bazel-contrib/supply-chain gather_metadata pattern, this checks:
    1. package_metadata common attribute (from REPO.bazel default_package_metadata)
    2. applicable_licenses attribute (fallback for older configs)
    3. Direct package_info providers on the target itself

    Args:
        target: The target being visited
        ctx: The aspect context

    Returns:
        List containing BuildInfoMetadata provider
    """
    direct_versions = []

    # Approach 1: Check package_metadata common attribute (Bazel 5.4+)
    # This is set via REPO.bazel default_package_metadata in go_repository
    package_metadata = []
    if hasattr(ctx.rule.attr, "package_metadata"):
        attr_value = ctx.rule.attr.package_metadata
        if attr_value:
            package_metadata = attr_value if type(attr_value) == "list" else [attr_value]

    # Approach 2: Check applicable_licenses (legacy compatibility)
    if not package_metadata and hasattr(ctx.rule.attr, "applicable_licenses"):
        attr_value = ctx.rule.attr.applicable_licenses
        if attr_value:
            package_metadata = attr_value if type(attr_value) == "list" else [attr_value]

    # Collect metadata from transitive dependencies (supply-chain pattern)
    transitive_depsets = []

    # Traverse all attributes (supply-chain uses attr_aspects = ["*"])
    attrs = [attr for attr in dir(ctx.rule.attr)]
    for attr_name in attrs:
        # Skip private attributes
        if attr_name.startswith("_"):
            continue

        attr_value = getattr(ctx.rule.attr, attr_name)
        if not attr_value:
            continue

        # Handle both lists and single values
        deps = attr_value if type(attr_value) == "list" else [attr_value]

        for dep in deps:
            # Only process Target types
            if type(dep) != "Target":
                continue

            # Collect transitive BuildInfoMetadata
            if BuildInfoMetadata in dep:
                transitive_depsets.append(dep[BuildInfoMetadata].version_map)

    # Return using supply-chain memory optimization pattern
    if not direct_versions and not transitive_depsets:
        # No metadata at all, return empty provider
        return [BuildInfoMetadata(version_map = depset([]))]

    if not direct_versions and len(transitive_depsets) == 1:
        # Only one transitive depset, pass it up directly to save memory
        return [BuildInfoMetadata(version_map = transitive_depsets[0])]

    # Combine direct and transitive metadata
    return [BuildInfoMetadata(
        version_map = depset(
            direct = direct_versions,
            transitive = transitive_depsets,
        ),
    )]

buildinfo_aspect = aspect(
    doc = "Collects package_info metadata for Go buildInfo",
    implementation = _buildinfo_aspect_impl,
    attr_aspects = ["*"],  # Traverse all attributes (supply-chain pattern)
    provides = [BuildInfoMetadata],
    # Apply to generated targets including package_info
    apply_to_generating_rules = True,
)
