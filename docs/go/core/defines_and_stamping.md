## Defines and stamping

In order to provide build time information to go code without data files, we
support the concept of stamping.

Stamping asks the linker to substitute the value of a global variable with a
string determined at link time. Stamping only happens when linking a binary, not
when compiling a package. This means that changing a value results only in
re-linking, not re-compilation and thus does not cause cascading changes.

Link values are set in the `x_defs` attribute of any Go rule. This is a
map of string to string, where keys are the names of variables to substitute,
and values are the string to use. Keys may be names of variables in the package
being compiled, or they may be fully qualified names of variables in another
package.

These mappings are collected up across the entire transitive dependencies of a
binary. This means you can set a value using `x_defs` in a
`go_library`, and any binary that links that library will be stamped with that
value. You can also override stamp values from libraries using `x_defs`
on the `go_binary` rule if needed. The `--[no]stamp` option controls whether
stamping of workspace variables is enabled.

The values of the `x_defs` dictionary are subject to
[location expansion](https://bazel.build/reference/be/make-variables#predefined_label_variables).

**Example**

Suppose we have a small library that contains the current version.

``` go
package version

var Version = "redacted"
```

We can set the version in the `go_library` rule for this library.

``` bzl
go_library(
    name = "version",
    srcs = ["version.go"],
    importpath = "example.com/repo/version",
    x_defs = {"Version": "0.9"},
)
```

Binaries that depend on this library may also set this value.

``` bzl
go_binary(
    name = "cmd",
    srcs = ["main.go"],
    deps = ["//version"],
    x_defs = {"example.com/repo/version.Version": "0.9"},
)
```

### Stamping with the workspace status script

You can use values produced by the workspace status command in your link stamp.
To use this functionality, write a script that prints key-value pairs, separated
by spaces, one per line. For example:

``` bash
#!/usr/bin/env bash

echo STABLE_GIT_COMMIT $(git rev-parse HEAD)
```

***Note:*** stamping with keys that bazel designates as "stable" will trigger a
re-link when any stable key changes. Currently, in bazel, stable keys are
`BUILD_EMBED_LABEL`, `BUILD_USER`, `BUILD_HOST` and keys whose names start with
`STABLE_`. Stamping only with keys that are not stable keys will not trigger a
relink.

You can reference these in `x_defs` using curly braces.

``` bzl
go_binary(
    name = "cmd",
    srcs = ["main.go"],
    deps = ["//version"],
    x_defs = {"example.com/repo/version.Version": "{STABLE_GIT_COMMIT}"},
)
```

You can build using the status script using the `--workspace_status_command`
argument on the command line:

``` bash
$ bazel build --stamp --workspace_status_command=./status.sh //:cmd
```

### VCS build info

When `go_binary` and `go_test` targets are linked with stamping enabled,
`rules_go` also maps a small stable workspace status convention into Go build
info. These settings are visible through `runtime/debug.ReadBuildInfo` and
`go version -m`. They are only emitted when the linked target and its recorded
main module both come from the main repository; stamped binaries built from
external repositories, targets without main module metadata, and local wrappers
around external main modules omit `vcs.*` settings.

Including test binaries is intentional. Go's own toolchain regenerates build
information for test binaries and includes VCS metadata when `-buildvcs=true`.
Bazel's `--stamp` flag is likewise an explicit provenance opt-in. This differs
from Go's default `-buildvcs=auto` mode, which omits VCS metadata from test
binaries.

***Warning:*** avoid enabling VCS stamping for routine development test runs.
The current commit hash becomes part of every eligible stamped `go_test`
binary. Creating a commit, amending a commit message, or rebasing changes that
hash, which relinks those test binaries and invalidates their cached test
results even when their source code did not change. This does not invalidate Go
compilation outputs, but it can substantially reduce test cache hits across a
workspace. Keep stamping disabled for normal development and enable it for
release or provenance-verification builds where embedding the exact revision is
worth the cache cost.

Target and main-module provenance are checked separately. A local target can
embed a main package whose metadata came from an external repository; in that
case the target is local, but the module recorded in `BuildInfo.Main` is not.
Main-module provenance follows the metadata source that supplied `Main.Path`,
including embedded libraries and `package_metadata` / `applicable_licenses`,
and is derived from the metadata file's owning label at link time.

The workspace status names mirror Go's build info names, with Bazel's
`STABLE_` prefix and underscores in place of dots.

| Workspace status key | Go build info setting | Notes |
| --- | --- | --- |
| `STABLE_VCS` | `vcs` | Required and must be non-empty before any `vcs.*` settings are emitted. |
| `STABLE_VCS_REVISION` | `vcs.revision` | Optional when non-empty. |
| `STABLE_VCS_TIME` | `vcs.time` | Optional; must be in `RFC3339Nano` format and is normalized to UTC. |
| `STABLE_VCS_MODIFIED` | `vcs.modified` | Optional; must be `true` or `false`. |

Invalid or missing optional values are ignored individually.

``` bash
#!/usr/bin/env bash

echo "STABLE_VCS git"
echo "STABLE_VCS_REVISION $(git rev-parse HEAD)"
echo "STABLE_VCS_TIME $(git show -s --format=%cI HEAD)"
if test -z "$(git status --porcelain --untracked-files=normal)"; then
  echo "STABLE_VCS_MODIFIED false"
else
  echo "STABLE_VCS_MODIFIED true"
fi
```
