# Vim + `rules_go`

`rules_go` has support for integration with text editors, see [Editor and tool
integration](https://github.com/bazelbuild/rules_go/wiki/Editor-and-tool-integration).
This document describes integration with Vim, which will provide semantic
autocompletion for Bazel-generated Go files, among other things.

## Setup

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
