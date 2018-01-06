package main

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"

	"github.com/jimmyfrasche/closed"
	"github.com/jimmyfrasche/closed/cmds/internal/closedutil"
	"github.com/jimmyfrasche/closed/cmds/internal/tools"
)

type Type struct {
	Name          string
	FileSet       *token.FileSet
	Files         []*ast.File
	Types         *types.Package
	DefinedInFile string
	T             closed.Type
	Pkg           *Package
}

func LoadType(importPath, typeName string) (*Type, error) {
	bi, err := build.Import(importPath, "", 0)
	if err != nil {
		return nil, err
	}

	T := &Type{
		Name:    typeName,
		FileSet: token.NewFileSet(),
	}

	packages, err := parser.ParseDir(T.FileSet, bi.Dir, tools.MakeFileCheck(bi.GoFiles), parser.ParseComments)
	if err != nil {
		return nil, err
	}

	T.Files = tools.FilesToSlice(packages[bi.Name])

	cfg := types.Config{
		Importer: importer.Default(),
	}

	T.Types, err = cfg.Check(bi.ImportPath, T.FileSet, T.Files, nil)
	if err != nil {
		return nil, err
	}

	T.Pkg = &Package{
		Name:       T.Types.Name(),
		ImportPath: T.Types.Path(),
	}

	obj := T.Types.Scope().Lookup(T.Name)
	if obj == nil {
		return nil, fmt.Errorf("could not find type %s", T.Name)
	}

	typ, ok := obj.(*types.TypeName)
	if !ok {
		return nil, fmt.Errorf("%s is not a defined type", T.Name)
	}

	pos := T.FileSet.Position(typ.Pos())
	if !pos.IsValid() {
		return nil, fmt.Errorf("could not find file containing %s", T.Name)
	}
	T.DefinedInFile = pos.Filename

	ts, err := closed.InPackage(T.FileSet, T.Files, T.Types)
	if err != nil {
		return nil, err
	}

	T.T = find(typ, ts)
	if T.T == nil {
		return nil, fmt.Errorf("%s is not recognized as a closed type", T.Name)
	}

	return T, nil
}

//find t in vs.
func find(t *types.TypeName, vs []closed.Type) closed.Type {
	for _, v := range vs {
		for _, n := range v.Types() {
			if t == n {
				return v
			}
		}
	}
	return nil
}

//mustFunc returns true if c does not allow methods.
func mustFunc(c closed.Type) bool {
	switch c.(type) {
	case *closed.Interface, *closed.EmptySum:
		return true
	}
	return false
}

//externalOkay ensures T can be validated outside its defining package.
func externalOkay(T *Type) error {
	//T is from another package, need to make sure we have access to it.
	if !ast.IsExported(T.Name) {
		if tn := closedutil.FirstExportedTypeName(T.T.Types()); tn != nil {
			//We could silently replace T.Name with tn.Name() here
			//but is it better that the go generate declaration
			//in the source code be clear than for this generator
			//to be clever
			return fmt.Errorf("%s is not exported from %q, but has exported names to use such as %s", T.Name, T.Pkg.ImportPath, tn.Name())
		}
		return fmt.Errorf("%s is not exported from %q", T.Name, T.Pkg.ImportPath)
	}

	if !closedutil.ExternallyExhaustible(T.T) {
		return fmt.Errorf("%s cannot be validated outside of %q", T.Name, T.Pkg.ImportPath)
	}

	return nil
}
