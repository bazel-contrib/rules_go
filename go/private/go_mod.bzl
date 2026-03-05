# Copyright 2025 The Bazel Authors. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

def version_from_go_mod(module_ctx, go_mod_label):
    """Returns a version string from a go.mod file.

    Args:
       module_ctx: a https://bazel.build/rules/lib/module_ctx object passed
            from the MODULE.bazel call.
       go_mod_label: a Label for a `go.mod` file.

    Returns:
      a string containing the version of the Go SDK defined in go.mod
    """
    return _version_from_go_file(module_ctx, go_mod_label, "go.mod", "1.16")

def version_from_go_work(module_ctx, go_work_label):
    """Returns a version string from a go.work file.

    Args:
       module_ctx: a https://bazel.build/rules/lib/module_ctx object passed
            from the MODULE.bazel call.
       go_work_label: a Label for a `go.work` file.

    Returns:
      a string containing the version of the Go SDK defined in go.work
    """
    return _version_from_go_file(module_ctx, go_work_label, "go.work", "1.18")

def _version_from_go_file(module_ctx, file_label, expected_filename, default_version):
    _check_filename(file_label.name, expected_filename)
    file_path = module_ctx.path(file_label)
    file_content = module_ctx.read(file_path)

    state = {
        "toolchain": None,
        "go": None,
    }

    current_directive = None
    for line_no, line in enumerate(file_content.splitlines(), 1):
        tokens, _ = _tokenize_line(line, file_path, line_no)
        if not tokens:
            continue

        if not current_directive:
            if tokens[0] == "go":
                _validate_go_version(file_path, state, tokens, line_no)
                state["go"] = tokens[1]

            if tokens[0] == "toolchain":
                _validate_toolchain_version(file_path, state, tokens, line_no)
                state["toolchain"] = tokens[1][len("go"):].strip()

            if tokens[1] == "(":
                current_directive = tokens[0]
                if len(tokens) > 2:
                    fail("{}:{}: unexpected token '{}' after '('".format(file_path, line_no, tokens[2]))
                continue
        elif tokens[0] == ")":
            current_directive = None
            if len(tokens) > 1:
                fail("{}:{}: unexpected token '{}' after ')'".format(file_path, line_no, tokens[1]))
            continue

    version = state["toolchain"]
    if not version:
        # https://go.dev/doc/toolchain: "a go.mod that says go 1.21.0 with no toolchain line is interpreted as if it had a toolchain go1.21.0 line."
        version = state["go"]
    if not version:
        # https://go.dev/doc/toolchain#config: "For compatibility reasons, if the go line is omitted from a go.mod file,
        # the module is considered to have an implicit go 1.16 line, and if the go line is omitted from a go.work file,
        # the workspace is considered to have an implicit go 1.18 line."
        version = default_version

    return version

def _tokenize_line(line, path, line_no):
    tokens = []
    r = line
    for _ in range(len(line)):
        r = r.strip()
        if not r:
            break

        if r[0] == "`":
            end = r.find("`", 1)
            if end == -1:
                fail("{}:{}: unterminated raw string".format(path, line_no))

            tokens.append(r[1:end])
            r = r[end + 1:]

        elif r[0] == "\"":
            value = ""
            escaped = False
            found_end = False
            pos = 0
            for pos in range(1, len(r)):
                c = r[pos]

                if escaped:
                    value += c
                    escaped = False
                    continue

                if c == "\\":
                    escaped = True
                    continue

                if c == "\"":
                    found_end = True
                    break

                value += c

            if not found_end:
                fail("{}:{}: unterminated interpreted string".format(path, line_no))

            tokens.append(value)
            r = r[pos + 1:]

        elif r.startswith("//"):
            # A comment always ends the current line
            return tokens, r[len("//"):].strip()

        else:
            token, _, r = r.partition(" ")
            tokens.append(token)

    return tokens, None

def _check_filename(name, expected):
    if name != expected:
        fail("go_sdk.from_file requires a '{}' file, not '{}'".format(expected, name))

def _validate_go_version(path, state, tokens, line_no):
    if len(tokens) == 1:
        fail("{}:{}: expected another token after 'go'".format(path, line_no))
    if state["go"] != None:
        fail("{}:{}: unexpected second 'go' directive".format(path, line_no))
    if len(tokens) > 2:
        fail("{}:{}: unexpected token '{}' after '{}'".format(path, line_no, tokens[2], tokens[1]))

def _validate_toolchain_version(path, state, tokens, line_no):
    if len(tokens) == 1:
        fail("{}:{}: expected another token after 'toolchain'".format(path, line_no))
    if state["toolchain"] != None:
        fail("{}:{}: unexpected second 'toolchain' directive".format(path, line_no))
    if len(tokens) > 2:
        fail("{}:{}: unexpected token '{}' after '{}'".format(path, line_no, tokens[2], tokens[1]))
    if not tokens[1].startswith("go"):
        fail("{}:{}: expected toolchain version to start with 'go', not '{}'".format(path, line_no, tokens[1]))
