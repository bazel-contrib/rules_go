#!/bin/bash
# Generates, for a GOFIPS140 snapshot build, two files from the snapshot zip
# under lib/fips140 (which Starlark cannot read):
#   - fips_out: the versioned GOFIPS140 snapshot package import paths only. The
#     stdlib builder reads this verbatim to know which packages to place into
#     pkg/ (see installFIPSSnapshotArchives). This file can be removed once
#     upstream Go installs GOFIPS140 packages into the package tree instead of
#     the build cache (golang/go#76225).
#   - out: packages.txt = the normal standard library package list plus the
#     versioned FIPS packages, consumed by the linker's importcfg.
#
# Producing both here keeps a single enumeration (no re-derivation in Go).
set -euo pipefail

normal="$1"   # file: normal stdlib package list
out="$2"      # file: combined packages.txt to write
libdir="$3"   # dir: GOROOT/lib/fips140
gofips="$4"   # GOFIPS140 value: a version (e.g. v1.0.0) or alias (certified, inprocess), resolved via lib/fips140/<value>.txt
fips_out="$5" # file: versioned FIPS package list to write

# Resolve an alias (e.g. v1.0.0 -> v1.0.0-c2097c7c) when an alias file exists.
if [ -f "$libdir/$gofips.txt" ]; then
  ver="$(cat "$libdir/$gofips.txt")"
else
  ver="$gofips"
fi

# Zip entries look like golang.org/fips140@<ver>/fips140/<ver>/<pkg>/<file>.go,
# which map to the import path crypto/internal/fips140/<ver>/<pkg>.
unzip -Z1 "$libdir/$ver.zip" \
  | grep '\.go$' \
  | grep -v '_test\.go$' \
  | grep -E '^golang\.org/fips140@[^/]+/fips140/' \
  | sed -E 's#^golang.org/fips140@[^/]+/fips140/#crypto/internal/fips140/#; s#/[^/]+\.go$##' \
  | LC_ALL=C sort -u > "$fips_out"

# packages.txt = normal stdlib list + the FIPS packages.
LC_ALL=C sort -u "$normal" "$fips_out" > "$out"
