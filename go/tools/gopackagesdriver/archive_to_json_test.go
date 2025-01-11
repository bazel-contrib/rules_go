package main

import (
	"errors"
	"io"
	"strings"
	"testing"
)

type MockedParams struct {
	ReaderContents     string
	GetterErr          error
	ExpectedParamsPath string
}

// Depending on the arguments provided
// tokenize bazel arguments from the arguments themselves or from a file.
func TestBazelParser(t *testing.T) {
	tests := []struct {
		name                  string
		args                  string
		expectParseFromParams *MockedParams
		expectedBazelArgs     map[string][]string
		expectError           bool
	}{{
		name:                  "ParamAndExtraArgsShouldFail",
		args:                  "@param.txt foo bar",
		expectParseFromParams: nil,
		expectedBazelArgs:     nil,
		expectError:           true,
	}, {
		name:                  "FromArgsOk",
		args:                  "--id foo bar --pkg-path bar baz",
		expectParseFromParams: nil,
		expectedBazelArgs: map[string][]string{
			"--id":       {"foo", "bar"},
			"--pkg-path": {"bar", "baz"},
		},
		expectError: false,
	}, {
		name: "FromParamsOk",
		args: "@params.txt",
		expectParseFromParams: &MockedParams{
			ReaderContents:     "--arg1\nfoo\nbar\nbaz\n--arg2\nbar\nbaz",
			GetterErr:          nil,
			ExpectedParamsPath: "params.txt",
		},
		expectedBazelArgs: map[string][]string{
			"--arg1": {"foo", "bar", "baz"},
			"--arg2": {"bar", "baz"},
		},
		expectError: false,
	}, {
		name: "FromParamsUnknownFirstArg",
		args: "@params.txt",
		expectParseFromParams: &MockedParams{
			ReaderContents:     "--arg3\n--arg2\nbar\nbaz",
			GetterErr:          nil,
			ExpectedParamsPath: "params.txt",
		},
		expectError: true,
	}, {
		name: "FromParamsCannotGetReader",
		args: "@params.txt",
		expectParseFromParams: &MockedParams{
			ReaderContents:     "",
			GetterErr:          errors.New("an error"),
			ExpectedParamsPath: "params.txt",
		},
		expectedBazelArgs: nil,
		expectError:       true,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actualArgs := make(map[string][]string)
			parser := NewBazelArgsParser()
			for k := range tc.expectedBazelArgs {
				flag := string(k)
				parser.WithArg(k, func(value string) {
					actualArgs[flag] = append(actualArgs[flag], value)
				})
			}
			paramsReaderGetterCallsCount := 0
			var requestedParamsPath string
			parser.WithParamsReaderGetter(func(paramsPath string) (io.Reader, error) {
				paramsReaderGetterCallsCount++
				if tc.expectParseFromParams != nil {
					requestedParamsPath = paramsPath
					getterError := tc.expectParseFromParams.GetterErr
					if getterError != nil {
						return nil, getterError
					}
					return strings.NewReader(tc.expectParseFromParams.ReaderContents), nil
				}
				panic("unexpected test path")
			})
			err := parser.Parse(strings.Fields(tc.args))
			if tc.expectError && err == nil {
				t.Error("expected error")
			}

			// Expecting a call to params getter.
			if tc.expectParseFromParams != nil {
				if paramsReaderGetterCallsCount == 0 {
					t.Error("expected call to params reader")
				}
				if requestedParamsPath != tc.expectParseFromParams.ExpectedParamsPath {
					t.Errorf("expected '%s' got: %s",
						tc.expectParseFromParams.ExpectedParamsPath, requestedParamsPath)
				}
			}

			// Check the parsed args are identical to the expected.
			for expectedFlag, expectedValues := range tc.expectedBazelArgs {
				actualValues, ok := actualArgs[expectedFlag]
				if !ok {
					t.Errorf("expecting to find '%s'", expectedFlag)
				}
				expectedValuesCount := len(expectedValues)
				actualValuesCount := len(actualValues)
				if len(expectedValues) != len(actualValues) {
					t.Errorf("for flag %s expecting to %d values got %d",
						expectedFlag, expectedValuesCount, actualValuesCount)
				}
			}
		})
	}
}

// Depending on the arguments provided
// tokenize bazel arguments from the arguments themselves or from a file.
func TestEscapedFieldsArgParser(t *testing.T) {
	tests := []struct {
		value          string
		expectedFields []string
	}{{
		value:          "foo\\\\\\==\\\\ba\\=r",
		expectedFields: []string{"foo\\=", "\\ba=r"},
	}, {
		value:          "\\=baz\\==\\=\\\\",
		expectedFields: []string{"=baz=", "=\\"},
	}}
	for _, tc := range tests {
		t.Run(tc.value, func(t *testing.T) {
			fields := splitEscapedString(tc.value, '=', '\\')
			if len(fields) != len(tc.expectedFields) {
				t.Errorf("invalid number of fields '%d', expected: '%d'", len(fields), len(tc.expectedFields))
				return
			}
			for i, expectedField := range tc.expectedFields {
				field := fields[i]
				if field != expectedField {
					t.Errorf("invalid field '%s', expected: '%s'", field, expectedField)
				}
			}
		})
	}
}
