package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path"

	"github.com/jimmyfrasche/closed"
	"github.com/jimmyfrasche/closed/cmds/internal/closedutil"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/loader"
)

type importMap map[string]string

func (i importMap) Name(p *types.Package) string {
	imp := i[p.Path()]
	switch imp {
	case "":
		return p.Name()
	case ".":
		return ""
	default:
		return imp
	}
}

func addImportsAndGetLocalPackageNames(fs *token.FileSet, f *ast.File, ct closed.Type, pkg, dpkg *loader.PackageInfo, sw ast.Stmt, prog *loader.Program) (importMap, error) {
	present := importMap(importsOfFile(f))
	inT, err := closedutil.ImportsOf(ct)
	if err != nil {
		return nil, err
	}

	if pkg.Pkg == dpkg.Pkg {
		delete(inT, pkg.Pkg.Path())
	}

	for p := range inT {
		//import already exists
		if _, ok := present[p]; ok {
			delete(inT, p)
		}
	}

	if len(inT) == 0 {
		return present, nil
	}

	s := dpkg.Info.Scopes[sw]

	for imp := range inT {
		P := prog.Package(imp).Pkg.Name() //In all cases p must be in trans. deps. of dpkg
		if s.Lookup(P) != nil {
			return nil, fmt.Errorf("cannot import %q, %s already in scope", imp, P)
		}
		if P == path.Base(imp) {
			astutil.AddImport(fs, f, imp)
		} else {
			astutil.AddNamedImport(fs, f, P, imp)
		}
		present[imp] = P
	}

	return present, nil
}
