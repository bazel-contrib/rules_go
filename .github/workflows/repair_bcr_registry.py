#!/usr/bin/env python3

import argparse
import base64
import hashlib
import json
from pathlib import Path


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", type=Path, required=True)
    parser.add_argument("--registry", type=Path, required=True)
    parser.add_argument("--tag", required=True)
    args = parser.parse_args()

    if args.tag != "v0.63.0":
        raise ValueError(f"unexpected release tag: {args.tag}")

    version = args.tag.removeprefix("v")
    module_directory = args.registry / "modules" / "rules_go" / version
    source_module = (args.source / "MODULE.bazel").read_text()
    marker = '    repo_name = "io_bazel_rules_go",\n'
    if source_module.count(marker) != 1:
        raise ValueError("could not identify the rules_go module declaration")

    module_block = source_module.partition("\n)\n")[0]
    if "    version = " in module_block:
        raise ValueError("the tagged rules_go module already declares a version")

    corrected_module = source_module.replace(
        marker, marker + f'    version = "{version}",\n', 1
    )
    expected_dependency = 'bazel_dep(name = "bazel_features", version = "1.36.0",'
    if expected_dependency not in corrected_module:
        raise ValueError("the released bazel_features dependency is unexpected")
    (module_directory / "MODULE.bazel").write_text(corrected_module)

    previous_patch = (
        args.registry
        / "modules"
        / "rules_go"
        / "0.62.0"
        / "patches"
        / "module_dot_bazel_version.patch"
    ).read_text()
    if previous_patch.count('"0.62.0"') != 1:
        raise ValueError("the previous module-version patch is unexpected")

    corrected_patch = previous_patch.replace('"0.62.0"', f'"{version}"', 1)
    patch_path = module_directory / "patches" / "module_dot_bazel_version.patch"
    patch_path.write_text(corrected_patch)

    patch_digest = hashlib.sha256(patch_path.read_bytes()).digest()
    patch_integrity = "sha256-" + base64.b64encode(patch_digest).decode()
    expected_patch_integrity = "sha256-78UHBUbfVo/yuxxWI+c+Irs0ujETcjVVhr7V76rAkVg="
    if patch_integrity != expected_patch_integrity:
        raise ValueError(f"unexpected module-version patch integrity: {patch_integrity}")

    source_json_path = module_directory / "source.json"
    source_json = json.loads(source_json_path.read_text())
    expected_archive_integrity = "sha256-w+JTI3EJqy4qjTywdWiLmKbm/OQ9hJhJZI6OOoTyDW8="
    if source_json["integrity"] != expected_archive_integrity:
        raise ValueError("the published release archive integrity changed")
    source_json["patches"]["module_dot_bazel_version.patch"] = patch_integrity
    source_json_path.write_text(json.dumps(source_json, indent=4) + "\n")

    attestations_path = module_directory / "attestations.json"
    if not attestations_path.is_file():
        raise ValueError("the invalid registry attestation manifest is missing")
    attestations_path.unlink()

    print(f"Corrected rules_go module version: {version}")
    print(f"Preserved bazel_features version: 1.36.0")
    print(f"Preserved archive integrity: {expected_archive_integrity}")
    print(f"Updated module patch integrity: {patch_integrity}")


if __name__ == "__main__":
    main()
