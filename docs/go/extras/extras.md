<!-- Generated with Stardoc: http://skydoc.bazel.build -->

  [gazelle rule]: https://github.com/bazelbuild/bazel-gazelle#bazel-rule
  [golang/mock]: https://github.com/golang/mock
  [core go rules]: /docs/go/core/rules.md

# Extra rules

This is a collection of helper rules. These are not core to building a go binary, but are supplied
to make life a little easier.

## Contents
- [gazelle](#gazelle)
- [gomock](#gomock)

## Additional resources
- [gazelle rule]
- [golang/mock]
- [core go rules]

------------------------------------------------------------------------

gazelle
-------

This rule has moved. See [gazelle rule] in the Gazelle repository.

<a id="gomock"></a>

## gomock

<pre>
load("@rules_go//docs/go/extras:extras.bzl", "gomock")

gomock(<a href="#gomock-name">name</a>, <a href="#gomock-out">out</a>, <a href="#gomock-library">library</a>, <a href="#gomock-source_importpath">source_importpath</a>, <a href="#gomock-source">source</a>, <a href="#gomock-interfaces">interfaces</a>, <a href="#gomock-package">package</a>, <a href="#gomock-self_package">self_package</a>, <a href="#gomock-aux_files">aux_files</a>,
       <a href="#gomock-mockgen_tool">mockgen_tool</a>, <a href="#gomock-mockgen_args">mockgen_args</a>, <a href="#gomock-imports">imports</a>, <a href="#gomock-copyright_file">copyright_file</a>, <a href="#gomock-mock_names">mock_names</a>, <a href="#gomock-kwargs">kwargs</a>)
</pre>

Calls [mockgen](https://github.com/golang/mock) to generates a Go file containing mocks from the given library.

If `source` is given, the mocks are generated in source mode; otherwise in reflective mode.


**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="gomock-name"></a>name |  the target name.   |  none |
| <a id="gomock-out"></a>out |  the output Go file name.   |  none |
| <a id="gomock-library"></a>library |  the Go library to look into for the interfaces (reflective mode) or source (source mode). If running in source mode, you can specify source_importpath instead of this parameter.   |  `None` |
| <a id="gomock-source_importpath"></a>source_importpath |  the importpath for the source file. Alternative to passing library, which can lead to circular dependencies between mock and library targets. Only valid for source mode.   |  `""` |
| <a id="gomock-source"></a>source |  a Go file in the given `library`. If this is given, `gomock` will call mockgen in source mode to mock all interfaces in the file.   |  `None` |
| <a id="gomock-interfaces"></a>interfaces |  a list of interfaces in the given `library` to be mocked in reflective mode.   |  `[]` |
| <a id="gomock-package"></a>package |  the name of the package the generated mocks should be in. If not specified, uses mockgen's default. See [mockgen's -package](https://github.com/golang/mock#flags) for more information.   |  `""` |
| <a id="gomock-self_package"></a>self_package |  the full package import path for the generated code. The purpose of this flag is to prevent import cycles in the generated code by trying to include its own package. See [mockgen's -self_package](https://github.com/golang/mock#flags) for more information.   |  `""` |
| <a id="gomock-aux_files"></a>aux_files |  a map from source files to their package path. This only needed when `source` is provided. See [mockgen's -aux_files](https://github.com/golang/mock#flags) for more information.   |  `{}` |
| <a id="gomock-mockgen_tool"></a>mockgen_tool |  the mockgen tool to run.   |  `Label("@rules_go//extras/gomock:mockgen")` |
| <a id="gomock-mockgen_args"></a>mockgen_args |  additional arguments to pass to the mockgen tool.   |  `[]` |
| <a id="gomock-imports"></a>imports |  dictionary of name-path pairs of explicit imports to use. See [mockgen's -imports](https://github.com/golang/mock#flags) for more information.   |  `{}` |
| <a id="gomock-copyright_file"></a>copyright_file |  optional file containing copyright to prepend to the generated contents. See [mockgen's -copyright_file](https://github.com/golang/mock#flags) for more information.   |  `None` |
| <a id="gomock-mock_names"></a>mock_names |  dictionary of interface name to mock name pairs to change the output names of the mock objects. Mock names default to 'Mock' prepended to the name of the interface. See [mockgen's -mock_names](https://github.com/golang/mock#flags) for more information.   |  `{}` |
| <a id="gomock-kwargs"></a>kwargs |  <p align="center"> - </p>   |  none |


