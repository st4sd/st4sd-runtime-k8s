/*
	Copyright IBM Inc. All Rights Reserved.

	SPDX-License-Identifier: Apache-2.0

	Authors:
	  Vassilis Vassiliadis
*/

package v1alpha1

import (
	"regexp"
	"strings"
)

type SourcePathToTargetFileName struct {
	SourcePath string
	TargetName *string
}

// Unescapes a path
// Replaces \: with : and \\ with \
func Unescape(text string) string {
	text = strings.ReplaceAll(text, "\\:", ":")
	text = strings.ReplaceAll(text, "\\\\", "\\")

	return text
}

// SplitPathToSourcePathAndTargetName splits its operand to 2 strings (second can be nil)
// The format of @path is $sourcePath[:$targetName] where the character ':' is escaped
// as the string "\:". If there is no $targetName then ret.TargetName is nil
func SplitPathToSourcePathAndTargetName(path string) SourcePathToTargetFileName {
	strPattern := "(?P<SourcePath>([^:]|\\:)+[^\\\\]):(?P<TargetName>.+)"
	pattern := regexp.MustCompile(strPattern)
	ret := SourcePathToTargetFileName{}

	if pattern.MatchString(path) {
		match := pattern.FindStringSubmatch(path)
		idxTargetName := pattern.SubexpIndex("TargetName")
		ret.SourcePath = match[pattern.SubexpIndex("SourcePath")]

		if match[idxTargetName] != "" {
			ret.TargetName = &match[idxTargetName]
		}
	} else {
		ret.SourcePath = path
	}

	ret.SourcePath = Unescape(ret.SourcePath)
	if ret.TargetName != nil {
		tname := Unescape(*ret.TargetName)
		ret.TargetName = &tname
	}

	return ret
}
