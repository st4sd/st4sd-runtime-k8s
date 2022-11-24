/*
	Copyright IBM Inc. All Rights Reserved.

	SPDX-License-Identifier: Apache-2.0

	Authors:
	  Vassilis Vassiliadis
*/

package v1alpha1

import (
	"testing"
)

func constant_literal(s string) *string {
	return &s
}

// TestSplitPathToSourcePathAndTargetName tests that SplitPathToSourcePathAndTargetName
// splits $sourcePath:$renameTarget properly
func TestSplitPathToSourcePathAndTargetName(t *testing.T) {
	tests := map[string]SourcePathToTargetFileName{
		"/hello/world": {
			SourcePath: "/hello/world",
			TargetName: nil,
		},
		"/hello/world:other": {
			SourcePath: "/hello/world",
			TargetName: constant_literal("other"),
		},
		"\\:escaped": {
			SourcePath: ":escaped",
			TargetName: nil,
		},
		"\\:escaped:renamed": {
			SourcePath: ":escaped",
			TargetName: constant_literal("renamed"),
		},
	}

	for path, expected := range tests {
		// t.Log("Testing", path, "expected", expected)

		actual := SplitPathToSourcePathAndTargetName(path)
		if actual.SourcePath != expected.SourcePath {
			t.Error("Invalid SourcePath", "actual", actual, "expected", expected)
		}

		if (actual.TargetName != nil && expected.TargetName != nil &&
			*actual.TargetName != *expected.TargetName) ||
			(actual.TargetName == nil && expected.TargetName != nil) ||
			(actual.TargetName != nil && expected.TargetName == nil) {
			t.Error("Invalid TargetName", "actual", actual, "expected", expected)
		}
	}
}
