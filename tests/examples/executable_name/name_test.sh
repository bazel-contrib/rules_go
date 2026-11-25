result="$(${TEST_SRCDIR}/_main/tests/examples/executable_name/the_binary)"
expect="The executable ran!"
if [ "$result" != "$expect" ]; then
  echo "error: unexpected bazel exit code: want '$expect', got '$result'" >&2
  exit 1
fi
