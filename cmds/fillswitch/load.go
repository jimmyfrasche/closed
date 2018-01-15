package main

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"path/filepath"

	"golang.org/x/tools/go/loader"
)

func load(bc *build.Context, imp string) (*loader.Program, error) {
	impTest := imp + "_test"
	cfg := &loader.Config{
		ParserMode: parser.ParseComments,
		TypeCheckFuncBodies: func(p string) bool {
			return p == imp || p == impTest
		},
		Build: bc,
	}

	cfg.ImportWithTests(imp)
	return cfg.Load()
}

func getFile(fs *token.FileSet, file string, pkg *loader.PackageInfo) *ast.File {
	file = filepath.Base(file)
	for _, f := range pkg.Files {
		fname := filepath.Base(fs.File(f.Pos()).Name())
		if fname == file {
			return f
		}
	}
	return nil
}

func pkgWithFile(imp, file string, prog *loader.Program) (*loader.PackageInfo, *ast.File, error) {
	get := func(p string) (*loader.PackageInfo, *ast.File) {
		pkg := prog.Package(p)
		if pkg == nil {
			return nil, nil
		}
		f := getFile(prog.Fset, file, pkg)
		if f == nil {
			return nil, nil
		}
		return pkg, f
	}

	pkg, f := get(imp)
	if pkg != nil {
		return pkg, f, nil
	}

	//could be in an external test
	pkg, f = get(imp + "_test")
	if pkg != nil {
		return pkg, f, nil
	}

	return nil, nil, fmt.Errorf("could not find file %s in %q", file, imp)
}
