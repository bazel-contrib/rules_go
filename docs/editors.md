# General setup 

The `GOPACKAGESDRIVER` allows `gopls` and any package using `x/tools/packages` to
expose package data. Configuring the rules_go's packages driver is simple.

## 1. `gopls`
`gopls >= v0.6.10` ([released on Apr 13th 2021](https://github.com/golang/tools/releases/tag/gopls%2Fv0.6.10)) is required.

Install it in your path. If you have $GOBIN in your PATH, this should do the trick:
```
$ go install golang.org/x/tools/gopls@latest
```

If you are using Visual Studio Code, it should have been automatically updated by now.

## 2. Launcher script
Create a launcher script, say `tools/gopackagesdriver.sh`. If your repo is loading `rules_go` in its MODULE.bazel, give it these contents:

```bash
#!/usr/bin/env bash
exec bazel run -- @rules_go//go/tools/gopackagesdriver "${@}"
```

If your repo is still loading `rules_go` in the WORKSPACE file:

```bash
#!/usr/bin/env bash
exec bazel run -- @io_bazel_rules_go//go/tools/gopackagesdriver "${@}"
```

## 3. Editor Setup

You might want to replace `github.com/my/mypkg` with your package. When first opening
a file in the workspace, give the driver some time to load.

### Visual Studio Code
In the `.vscode/settings.json` of your workspace, you'll need to one of the two following JSON blobs, depending on if your repo is using MODULE.bazel or WORKSPACE to load rules_go (the difference is in the `go.goroot`). Also, for both of these, you'll need to edit some of the lines.

#### Bzlmod (MODULE.bazel file)
If you're using MODULE.bazel, use a JSON blob like this, after editing the following three lines:

1. In `build.directoryFilters` replace `mypkg` in `bazel-mypkg` with your repo's workspace name.
2. In `formatting.local` replace the import path there with the import path of your repo's code.
3. In `go.goroot`, replace `mymodule` with the name of your root module.

**NOTE:** The example below assumes you are using Bazel v8.0.0 or greater (or have the `--incompatible_use_plus_in_repo_names` flag enabled). If you are using an earlier version of Bazel (without that flag enabled), use this value for `go.goroot` instead: `${workspaceFolder}/bazel-${workspaceFolderBasename}/external/rules_go~~go_sdk~mymodule__download_0/`.

```jsonc
{
  // Settings for go/bazel are based on editor setup instructions at
  // https://github.com/bazelbuild/rules_go/wiki/Editor-setup#visual-studio-code
  "go.goroot": "${workspaceFolder}/bazel-${workspaceFolderBasename}/external/rules_go++go_sdk+mymodule__download_0/",
  "go.toolsEnvVars": {
    "GOPACKAGESDRIVER": "${workspaceFolder}/tools/gopackagesdriver.sh"
  },
  "go.enableCodeLens": {
    "runtest": false
  },
  "gopls": {
    "build.workspaceFiles": [
      "**/BUILD",
      "**/WORKSPACE",
      "**/*.{bzl,bazel}",
    ],
    "build.directoryFilters": [
      "-bazel-bin",
      "-bazel-out",
      "-bazel-testlogs",
      "-bazel-mypkg",
    ],
    "formatting.gofumpt": true,
    "formatting.local": "github.com/my/mypkg",
    "ui.completion.usePlaceholders": true,
    "ui.semanticTokens": true,
    "ui.codelenses": {
      "gc_details": false,
      "regenerate_cgo": false,
      "generate": false,
      "test": false,
      "tidy": false,
      "upgrade_dependency": false,
      "vendor": false
    },
  },
  "go.useLanguageServer": true,
  "go.buildOnSave": "off",
  "go.lintOnSave": "off",
  "go.vetOnSave": "off",
}
```

#### Pre-Bzlmod (WORKSPACE file)
If your repo is using WORKSPACE to load `rules_go`, use a JSON blob like this, after editing the following two lines:

1. In `build.directoryFilters` replace `mypkg` in `bazel-mypkg` with your repo's workspace name.
2. In `formatting.local` replace the import path there with the import path of your repo's code.

```jsonc
{
  // Settings for go/bazel are based on editor setup instructions at
  // https://github.com/bazelbuild/rules_go/wiki/Editor-setup#visual-studio-code
  "go.goroot": "${workspaceFolder}/bazel-${workspaceFolderBasename}/external/rules_go++go_sdk+go_sdk/",
  "go.toolsEnvVars": {
    "GOPACKAGESDRIVER": "${workspaceFolder}/tools/gopackagesdriver.sh"
  },
  "go.enableCodeLens": {
    "runtest": false
  },
  "gopls": {
    "build.workspaceFiles": [
      "**/BUILD",
      "**/WORKSPACE",
      "**/*.{bzl,bazel}",
    ],
    "build.directoryFilters": [
      "-bazel-bin",
      "-bazel-out",
      "-bazel-testlogs",
      "-bazel-mypkg",
    ],
    "formatting.gofumpt": true,
    "formatting.local": "github.com/my/mypkg",
    "ui.completion.usePlaceholders": true,
    "ui.semanticTokens": true,
    "ui.codelenses": {
      "gc_details": false,
      "regenerate_cgo": false,
      "generate": false,
      "test": false,
      "tidy": false,
      "upgrade_dependency": false,
      "vendor": false
    },
  },
  "go.useLanguageServer": true,
  "go.buildOnSave": "off",
  "go.lintOnSave": "off",
  "go.vetOnSave": "off",
}
```

### Neovim

```lua
nvim_lsp.gopls.setup {
  on_attach = on_attach,
  settings = {
    gopls = {
      workspaceFiles = {
        "**/BUILD",
        "**/WORKSPACE",
        "**/*.{bzl,bazel}",
      },
      env = {
        GOPACKAGESDRIVER = './tools/gopackagesdriver.sh'
      },
      directoryFilters = {
        "-bazel-bin",
        "-bazel-out",
        "-bazel-testlogs",
        "-bazel-mypkg",
      },
      ...
    },
  },
}
```

### Vim

1. Install [vim-go](https://github.com/fatih/vim-go), a Vim plugin for Go
   development with support for `gopls`.

2. Follow the instructions from [Editor
   setup](https://github.com/bazelbuild/rules_go/wiki/Editor-setup#3-editor-setup)
   for installing `gopls` and adding a launcher script.
   * Note that `gopls` should already be installed as part of installing `vim-go`.

3. Add the following to your `.vimrc`:

      ```vim
      function! MaybeSetGoPackagesDriver()
        " Start at the current directory and see if there's a WORKSPACE file in the
        " current directory or any parent. If we find one, check if there's a
        " gopackagesdriver.sh in a tools/ directory, and point our
        " GOPACKAGESDRIVER env var at it.
        let l:dir = getcwd()
        while l:dir != "/"
          if filereadable(simplify(join([l:dir, 'WORKSPACE'], '/')))
            let l:maybe_driver_path = simplify(join([l:dir, 'tools/gopackagesdriver.sh'], '/'))
            if filereadable(l:maybe_driver_path)
              let $GOPACKAGESDRIVER = l:maybe_driver_path
              break
            end
          end
          let l:dir = fnamemodify(l:dir, ':h')
        endwhile
      endfunction

      call MaybeSetGoPackagesDriver()

      " See https://github.com/golang/tools/blob/master/gopls/doc/settings.md
      let g:go_gopls_settings = {
        \ 'build.workspaceFiles': [
          \ '**/BUILD',
          \ '**/WORKSPACE',
          \ '**/*.{bzl,bazel}',
        \ ], 
        \ 'build.directoryFilters': [
          \ '-bazel-bin',
          \ '-bazel-out',
          \ '-bazel-testlogs',
          \ '-bazel-mypkg',
        \ ],
        \ 'ui.completion.usePlaceholders': v:true,
        \ 'ui.semanticTokens': v:true,
        \ 'ui.codelenses': {
          \ 'gc_details': v:false,
          \ 'regenerate_cgo': v:false,
          \ 'generate': v:false,
          \ 'test': v:false,
          \ 'tidy': v:false,
          \ 'upgrade_dependency': v:false,
          \ 'vendor': v:false,
        \ },
      \ }
      ```

    * You'll want to replace `-bazel-mypkg` with your package.
    * If you've put your `gopackagesdriver.sh` script somewhere other than
      `tools/gopackagesdriver.sh`, you'll need to update
      `MaybeSetGoPackagesDriver` accordingly.

### Sublime Text
Here is a sample `.sublime-project`:
```json
{
  "folders": [
    {
      "path": ".",
      "folder_exclude_patterns": ["bazel-*"]
    }
  ],
  "settings": {
    "lsp_format_on_save": true,
    "LSP": {
      "gopls": {
        "enabled": true,
        "command": ["~/go/bin/gopls"],
        "selector": "source.go",
        "settings": {
          "gopls.workspaceFiles": [
            "**/BUILD",
            "**/WORKSPACE",
            "**/*.{bzl,bazel}",
          ],
          "gopls.directoryFilters": [
            "-bazel-bin",
            "-bazel-out",
            "-bazel-testlogs",
            "-bazel-mypkg",
          ],
          "gopls.allowImplicitNetworkAccess": false,
          "gopls.usePlaceholders": true,
          "gopls.gofumpt": true,
          "gopls.local": "github.com/my/mypkg",
          "gopls.semanticTokens": true,
          "gopls.codelenses": {
            "gc_details": false,
            "regenerate_cgo": false,
            "generate": false,
            "test": false,
            "tidy": false,
            "upgrade_dependency": false,
            "vendor": false
          }
        },
        "env": {
          "GOPACKAGESDRIVER": "./tools/gopackagesdriver.sh"
        }
      }
    }
  }
}
```

### Helix Editor
Here is a sample `.helix/languages.toml`:
```toml
[language-server.gopls.config]
"build.workspaceFiles" = ["**/BUILD", "**/WORKSPACE", "**/*.{bzl,bazel}"]
"build.directoryFilters" = [
  "-bazel-bin",
  "-bazel-out",
  "-bazel-testlogs",
  "-bazel-mypkg",
]
"formatting.gofumpt" = true
"formatting.local" = "github.com/my/mypkg"
"ui.completion.usePlaceholders" = true
"ui.semanticTokens" = true
"ui.codelenses" = { gc_details = false, generate = false, regenerate_cgo = false, test = false, tidy = false, upgrade_dependency = false, vendor = false }
```

### Zed

In `.zed/settings.json` (top right-hand hamburger menu -> "Open Project Settings")

```jsonc
{
  "lsp": {
    "gopls": {
      "binary": {
        "env": {
          // See https://github.com/bazel-contrib/rules_go/wiki/Editor-setup
          "GOPACKAGESDRIVER": "./tools/gopackagesdriver.sh"
        }
      }
    }
  }
}
```

Other Go-related settings at https://zed.dev/docs/languages/go

## Environment variables
The package driver has a few environment configuration variables, although most won't
need to configure them:
- `GOPACKAGESDRIVER_BAZEL`: bazel binary, defaults to `bazel`
- `BUILD_WORKSPACE_DIRECTORY`: directory of the bazel workspace (auto detected when using a launcher script because it invokes `bazel run`)
- `GOPACKAGESDRIVER_BAZEL_FLAGS` which will be passed to `bazel` invocations
- `GOPACKAGESDRIVER_BAZEL_QUERY_FLAGS` which will be passed to `bazel query`
  invocations
- `GOPACKAGESDRIVER_BAZEL_QUERY_SCOPE` which specifies the scope for `importpath` queries (since `gopls` only issues `file=` queries, so **use if you know what you're doing!**)
- `GOPACKAGESDRIVER_BAZEL_BUILD_FLAGS` which will be passed to `bazel build`
  invocations

## Debugging
It is possible to debug driver issues by calling it directly and looking at the errors
in the outputs:
```
$ echo {} | ./tools/gopackagesdriver.sh file=relative/path.go
```

## Limitations
- CGo completion may not work, but at least it's not explicitly supported.
- Errors are not handled
