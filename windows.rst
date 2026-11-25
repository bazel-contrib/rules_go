Using rules_go on Windows
=========================

.. _Installing Bazel on Windows: https://docs.bazel.build/versions/master/install-windows.html

This document provides a list of instructions for setting up Bazel to build
Go code on a Windows computer.

Most of the difficulty here is installing a compatible C/C++ toolchain. Cgo
only works with GCC and clang toolchains, so MSVC cannot be used. This is a
Go limitation, not a Bazel or rules_go problem: cgo determines types of
definitions by parsing error messages that GCC emits when compiling generated
files.

See also `Installing Bazel on Windows`_, the official instructions for
installing Bazel.

Install and configure dependencies
----------------------------------

* Install msys2 from https://www.msys2.org/. This is needed to provide a bash
  environment for Bazel.

  * Follow the installation directions to the end, including
    running ``pacman -Syu`` and ``pacman -Su`` in the msys2 shell.

* Install additional msys2 tools.

  * Run ``pacman -S mingw-w64-x86_64-gcc``. GCC is needed if you plan to build
    any cgo code. MSVC will not work with cgo. This is a Go limitation, not a
    Bazel limitation. cgo determines types of definitions by compiling specially
    crafted C files and parsing error messages. GCC or clang are specifically
    needed for this.
  * Run ``pacman -S patch``. ``patch`` is needed by ``git_repository`` and
    ``http_archive`` dependencies declared by rules_go. We use it to add
    and modify build files.

* Add ``C:\msys64\usr\bin`` to ``PATH`` in order to locate ``patch`` and
  other DLLs.
* Add ``C:\msys64\mingw64\bin`` to ``PATH`` in order to locate mingw DLLs.
  ``protoc`` and other host binaries will not run without these.
* Set the environment variable ``BAZEL_SH`` to ``C:\msys64\usr\bin\bash.exe``.
  Bazel needs this to run shell scripts.
* Set the environment variable ``CC`` to ``C:\msys64\mingw64\bin\gcc.exe``.
  Bazel uses this to configure the C/C++ toolchain.
* Install the MSVC++ redistributable from
  https://www.microsoft.com/en-us/download/details.aspx?id=48145.
  Bazel itself depends on this.
* Install Git from https://git-scm.com/download/win. The Git install should
  add the installed directory to your ``PATH`` automatically.

Install bazel
-------------

* Download Bazel from https://github.com/bazelbuild/bazel/releases.
* Move the binary to ``%APPDATA%\bin\bazel.exe``.
* Add that directory to ``PATH``.
* Confirm ``bazel version`` works.

Confirm C/C++ works
-------------------

Create a workspace with a simple ``cc_binary`` target.

.. code::

    -- WORKSPACE --

    -- BUILD.bazel --
    cc_binary(
        name = "hello",
        srcs = ["hello.c"],
    )

    -- hello.c --
    #include <stdio.h>

    int main() {
      printf("hello\n");
      return 0;
    }

To build with MinGW, run the command below. Add the ``-s`` flag to print
commands executed by Bazel to confirm MinGW is actually used.

.. code::

    bazel build --cpu=x64_windows --compiler=mingw-gcc //:hello

Bazel selects a C/C++ toolchain using the same platform and toolchain system
used by other rules. To ensure that the MinGW toolchain is registered, either
build against a ``platform``
target with the ``@bazel_tools//tools/cpp:mingw`` constraint such as
``@io_bazel_rules_go//go/toolchain:windows_amd64_cgo``, or define your own
target explicitly, as below:

.. code::

    platform(
        name = "windows_amd64_mingw",
        constraint_values = [
            "@bazel_tools//tools/cpp:mingw",
            "@platforms//cpu:x86_64",
            "@platforms//os:windows",
        ],
    )

You can build with the command below. This also ensures the MinGW toolchain is
registered (it is not, by default).

.. code::

    bazel build --extra_toolchains=@local_config_cc//:cc-toolchain-x64_windows_mingw --host_platform=//:windows_amd64_mingw --platforms=//:windows_amd64_mingw //:hello

You may want to add these flags to a ``.bazelrc`` file in your project root
directory or in your home directory.

.. code::

    build --cpu=x64_windows
    build --compiler=mingw-gcc
    build --extra_toolchains=@local_config_cc//:cc-toolchain-x64_windows_mingw
    build --host_platform=//:windows_amd64_mingw
    build --platforms=//:windows_amd64_mingw

Confirm Go works
----------------

* Copy boilerplate from rules_go.
* Confirm that you can run a pure Go "hello world" binary with
  ``bazel run //:target``
* Confirm you can run a cgo binary with the same set of flags and platforms
  used to build a C target above.
