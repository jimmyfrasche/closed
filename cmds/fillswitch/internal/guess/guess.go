// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//Package guess was copied out of the source for golang.org/x/tools/cmd/guru
package guess

import (
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"strings"
)

// ImportPath finds the package containing filename, and returns
// its source directory (an element of $GOPATH) and its import path
// relative to it.
//
// TODO(adonovan): what about _test.go files that are not part of the
// package?
//
func ImportPath(filename string, buildContext *build.Context) (srcdir, importPath string, err error) {
	absFile, err := filepath.Abs(filename)
	if err != nil {
		return "", "", fmt.Errorf("can't form absolute path of %s: %v", filename, err)
	}

	absFileDir := filepath.Dir(absFile)
	resolvedAbsFileDir, err := filepath.EvalSymlinks(absFileDir)
	if err != nil {
		return "", "", fmt.Errorf("can't evaluate symlinks of %s: %v", absFileDir, err)
	}

	segmentedAbsFileDir := segments(resolvedAbsFileDir)
	// Find the innermost directory in $GOPATH that encloses filename.
	minD := 1024
	for _, gopathDir := range buildContext.SrcDirs() {
		absDir, err := filepath.Abs(gopathDir)
		if err != nil {
			continue // e.g. non-existent dir on $GOPATH
		}
		resolvedAbsDir, err := filepath.EvalSymlinks(absDir)
		if err != nil {
			continue // e.g. non-existent dir on $GOPATH
		}

		d := prefixLen(segments(resolvedAbsDir), segmentedAbsFileDir)
		// If there are multiple matches,
		// prefer the innermost enclosing directory
		// (smallest d).
		if d >= 0 && d < minD {
			minD = d
			srcdir = gopathDir
			importPath = strings.Join(segmentedAbsFileDir[len(segmentedAbsFileDir)-minD:], string(os.PathSeparator))
		}
	}
	if srcdir == "" {
		return "", "", fmt.Errorf("directory %s is not beneath any of these GOROOT/GOPATH directories: %s",
			filepath.Dir(absFile), strings.Join(buildContext.SrcDirs(), ", "))
	}
	if importPath == "" {
		// This happens for e.g. $GOPATH/src/a.go, but
		// "" is not a valid path for (*go/build).Import.
		return "", "", fmt.Errorf("cannot load package in root of source directory %s", srcdir)
	}
	return srcdir, importPath, nil
}

func segments(path string) []string {
	return strings.Split(path, string(os.PathSeparator))
}

// prefixLen returns the length of the remainder of y if x is a prefix
// of y, a negative number otherwise.
func prefixLen(x, y []string) int {
	d := len(y) - len(x)
	if d >= 0 {
		for i := range x {
			if y[i] != x[i] {
				return -1 // not a prefix
			}
		}
	}
	return d
}
