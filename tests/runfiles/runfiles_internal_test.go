package runfiles

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const testRepoMapping = `my_module++ext1+repo1,my_module,my_module+
my_module++ext1+repo1,repo1,my_module++ext1+repo1
my_module++ext1+repo1,repo2,my_module++ext1+repo2
my_module++ext1+repo2,my_module,my_module+
my_module++ext1+repo2,repo1,my_module++ext1+repo1
my_module++ext1+repo2,repo2,my_module++ext1+repo2
my_module++ext2+*,my_module,my_module+
my_module++ext2+*,repo1,my_module++ext2+repo1
my_module++ext2+*,repo2,my_module++ext2+repo2
my_module++ext3+*,my_module,my_module+
my_module++ext+*,my_module,my_module+
`

func TestRepoMapping_Get(t *testing.T) {
	rm := getRepoMapping(t)

	for _, tc := range []struct {
		sourceRepo, targetRepoApparentName, expectedTargetRepo string
	}{
		{
			sourceRepo:             "my_module++ext1+repo1",
			targetRepoApparentName: "my_module",
			expectedTargetRepo:     "my_module+",
		},
		{
			sourceRepo:             "my_module++ext1+repo1",
			targetRepoApparentName: "repo1",
			expectedTargetRepo:     "my_module++ext1+repo1",
		},
		{
			sourceRepo:             "my_module++ext1+repo1",
			targetRepoApparentName: "repo2",
			expectedTargetRepo:     "my_module++ext1+repo2",
		},
		{
			sourceRepo:             "my_module++ext1+repo1",
			targetRepoApparentName: "non_existent_repo",
			expectedTargetRepo:     "",
		},
		{
			sourceRepo:             "my_module++ext1+repo2",
			targetRepoApparentName: "repo1",
			expectedTargetRepo:     "my_module++ext1+repo1",
		},
		{
			sourceRepo:             "my_module++ext2+repo1",
			targetRepoApparentName: "my_module",
			expectedTargetRepo:     "my_module+",
		},
		{
			sourceRepo:             "my_module++ext2+repo1",
			targetRepoApparentName: "repo1",
			expectedTargetRepo:     "my_module++ext2+repo1",
		},
		{
			sourceRepo:             "my_module++ext2+repo1",
			targetRepoApparentName: "repo2",
			expectedTargetRepo:     "my_module++ext2+repo2",
		},
		{
			sourceRepo:             "my_module++ext2+repo1",
			targetRepoApparentName: "non_existent_repo",
			expectedTargetRepo:     "",
		},
		{
			sourceRepo:             "my_module++ext2+repo2",
			targetRepoApparentName: "my_module",
			expectedTargetRepo:     "my_module+",
		},
		{
			sourceRepo:             "my_module++ext2+repo2",
			targetRepoApparentName: "repo1",
			expectedTargetRepo:     "my_module++ext2+repo1",
		},
		{
			sourceRepo:             "my_module++ext2+repo2",
			targetRepoApparentName: "repo2",
			expectedTargetRepo:     "my_module++ext2+repo2",
		},
		{
			sourceRepo:             "my_module++ext2+repo2",
			targetRepoApparentName: "non_existent_repo",
			expectedTargetRepo:     "",
		},
	} {
		t.Run(tc.sourceRepo+"->"+tc.targetRepoApparentName, func(t *testing.T) {
			targetRepo, found := rm.Get(repoMappingKey{tc.sourceRepo, tc.targetRepoApparentName})
			if targetRepo != tc.expectedTargetRepo {
				t.Fatalf("targetRepo differs: %q != %q", targetRepo, tc.expectedTargetRepo)
			}
			if found != (tc.expectedTargetRepo != "") {
				t.Fatalf("found differs: %v != %v", found, tc.expectedTargetRepo != "")
			}
		})
	}
}

func TestRepoMapping_ForEachVisible(t *testing.T) {
	rm := getRepoMapping(t)

	for _, tc := range []struct {
		sourceRepo                      string
		expectedTargetRepoApparentNames []string
		expectedTargetRepoDirectories   []string
	}{
		{
			sourceRepo:                      "my_module++ext1+repo1",
			expectedTargetRepoApparentNames: []string{"my_module", "repo1", "repo2"},
			expectedTargetRepoDirectories:   []string{"my_module+", "my_module++ext1+repo1", "my_module++ext1+repo2"},
		},
		{
			sourceRepo:                      "my_module++ext1+repo2",
			expectedTargetRepoApparentNames: []string{"my_module", "repo1", "repo2"},
			expectedTargetRepoDirectories:   []string{"my_module+", "my_module++ext1+repo1", "my_module++ext1+repo2"},
		},
		{
			sourceRepo:                      "my_module++ext2+repo1",
			expectedTargetRepoApparentNames: []string{"my_module", "repo1", "repo2"},
			expectedTargetRepoDirectories:   []string{"my_module+", "my_module++ext2+repo1", "my_module++ext2+repo2"},
		},
		{
			sourceRepo:                      "my_module++ext2+repo2",
			expectedTargetRepoApparentNames: []string{"my_module", "repo1", "repo2"},
			expectedTargetRepoDirectories:   []string{"my_module+", "my_module++ext2+repo1", "my_module++ext2+repo2"},
		},
		{
			sourceRepo: "non_existent_repo+",
		},
	} {
		t.Run(tc.sourceRepo, func(t *testing.T) {
			var targetRepoApparentNames, targetRepoDirectories []string
			rm.ForEachVisible(tc.sourceRepo, func(targetRepoApparentName, targetRepoDirectory string) {
				targetRepoApparentNames = append(targetRepoApparentNames, targetRepoApparentName)
				targetRepoDirectories = append(targetRepoDirectories, targetRepoDirectory)
			})
			slices.Sort(targetRepoApparentNames)
			slices.Sort(targetRepoDirectories)

			if len(targetRepoApparentNames) != len(tc.expectedTargetRepoApparentNames) {
				t.Fatalf("expected %d target repo apparent names, got %d: %v", len(tc.expectedTargetRepoApparentNames), len(targetRepoApparentNames), targetRepoApparentNames)
			}
			if len(targetRepoDirectories) != len(tc.expectedTargetRepoDirectories) {
				t.Fatalf("expected %d target repo directories, got %d: %v", len(tc.expectedTargetRepoDirectories), len(targetRepoDirectories), targetRepoDirectories)
			}
			for i, expectedName := range tc.expectedTargetRepoApparentNames {
				if targetRepoApparentNames[i] != expectedName {
					t.Errorf("target repo apparent name at index %d differs: %q != %q", i, targetRepoApparentNames[i], expectedName)
				}
			}
			for i, expectedDir := range tc.expectedTargetRepoDirectories {
				if targetRepoDirectories[i] != expectedDir {
					t.Errorf("target repo directory at index %d differs: %q != %q", i, targetRepoDirectories[i], expectedDir)
				}
			}
		})
	}
}

func getRepoMapping(t *testing.T) *repoMapping {
	t.Helper()
	tmp := t.TempDir()
	repoMappingFile := filepath.Join(tmp, "repo_mapping")
	err := os.WriteFile(repoMappingFile, []byte(testRepoMapping), 0755)
	if err != nil {
		t.Fatalf("failed to write repo mapping file: %s", err)
	}
	rm, err := parseRepoMapping(repoMappingFile)
	if err != nil {
		t.Fatalf("failed to parse repo mapping file: %s", err)
	}
	return rm
}
