#!/bin/bash
# Generates packages.txt for a GOFIPS140 snapshot build: the normal standard
# library package list plus the versioned FIPS module packages, listed from the
# snapshot zip under lib/fips140 (which Starlark cannot read).
#
# IMPORTANT: the FIPS enumeration below MUST produce the same package set as
# fipsSnapshotPackages in go/tools/builders/fips.go, which builds the matching
# archives. If the two diverge, the linker's importcfg (from this packages.txt)
# and the on-disk archives will mismatch at link time. fips_test.go pins them
# together.
set -euo pipefail

normal="$1"  # file: normal stdlib package list
out="$2"     # file: combined packages.txt to write
libdir="$3"  # dir: GOROOT/lib/fips140
gofips="$4"  # GOFIPS140 value: a version (e.g. v1.0.0) or alias (certified, inprocess), resolved via lib/fips140/<value>.txt

# Resolve an alias (e.g. v1.0.0 -> v1.0.0-c2097c7c) when an alias file exists.
if [ -f "$libdir/$gofips.txt" ]; then
  ver="$(cat "$libdir/$gofips.txt")"
else
  ver="$gofips"
fi

# Zip entries look like golang.org/fips140@<ver>/fips140/<ver>/<pkg>/<file>.go,
# which map to the import path crypto/internal/fips140/<ver>/<pkg>.
{
  cat "$normal"
  unzip -Z1 "$libdir/$ver.zip" \
    | grep '\.go$' \
    | grep -v '_test\.go$' \
    | grep -E '^golang\.org/fips140@[^/]+/fips140/' \
    | sed -E 's#^golang.org/fips140@[^/]+/fips140/#crypto/internal/fips140/#; s#/[^/]+\.go$##'
} | LC_ALL=C sort -u > "$out"
