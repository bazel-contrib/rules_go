#!/usr/bin/env bash

set -eo pipefail

IMPORTED_C_PATH="$1"
OUTPUT_DIR="$2"

case "$(uname -s)" in
  Linux*)
    cc -shared -o $OUTPUT_DIR/libimported.so $IMPORTED_C_PATH
    cc -shared -o $OUTPUT_DIR/libversioned.so.2 $IMPORTED_C_PATH
    ;;
  Darwin*)
    cc -shared -Wl,-install_name,@rpath/libimported.dylib -o $OUTPUT_DIR/libimported.dylib $IMPORTED_C_PATH
    # According to "Mac OS X For Unix Geeks", 4th Edition, Chapter 11, versioned dylib for macOS
    # should be libversioned.2.dylib.
    cc -shared -Wl,-install_name,@rpath/libversioned.2.dylib -o $OUTPUT_DIR/libversioned.2.dylib $IMPORTED_C_PATH
    # However, Oracle Instant Client was distributed as libclntsh.dylib.12.1 with a unversioed
    # symlink (https://www.oracle.com/database/technologies/instant-client/macos-intel-x86-downloads.html).
    # Let's cover this non-standard case as well.
    cc -shared -Wl,-install_name,@rpath/libversioned.dylib.2 -o $OUTPUT_DIR/libversioned.dylib.2 $IMPORTED_C_PATH
    cd $OUTPUT_DIR
    ln -fs libversioned.dylib.2 libversioned.dylib
    ;;
  *)
    echo "Unsupported OS: $(uname -s)" >&2
    exit 1
esac
