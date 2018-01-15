package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"github.com/jimmyfrasche/closed"
	"github.com/jimmyfrasche/closed/cmds/internal/closedutil"

	"golang.org/x/tools/go/loader"
)

func typeOf(theSwitch ast.Stmt, ti types.Info) (types.Type, error) {
	var t types.Type

	switch s := theSwitch.(type) {
	case *ast.TypeSwitchStmt:
		switch a := s.Assign.(type) {
		case *ast.AssignStmt:
			t = ti.TypeOf(a.Rhs[0].(*ast.TypeAssertExpr).X)

		case *ast.ExprStmt:
			t = ti.TypeOf(a.X.(*ast.TypeAssertExpr).X)

		default:
			panic(fmt.Errorf("unexpected %T in TypeSwitchStmt.Assign", s.Assign))
		}

	case *ast.SwitchStmt:
		if s.Tag == nil {
			return nil, errors.New("switch must switch over expression")
		}
		t = ti.TypeOf(s.Tag)
	}

	if t == nil {
		return nil, errors.New("could not derive type of switch statement")
	}
	return t, nil
}

func typeNameOf(t types.Type) (*types.TypeName, error) {
	nm, ok := t.(*types.Named)
	if !ok {
		return nil, fmt.Errorf("%s is not a named type", nm)
	}
	return nm.Obj(), nil
}

func definingPackage(t *types.TypeName, prog *loader.Program) (pkg *loader.PackageInfo, err error) {
	pkg = prog.Package(t.Pkg().Path())
	if pkg == nil {
		return nil, fmt.Errorf("could not load package for %s", t)
	}
	return pkg, nil
}

func getClosed(t *types.TypeName, fs *token.FileSet, pkg *loader.PackageInfo) (closed.Type, error) {
	closedTypes, err := closed.InPackage(fs, pkg.Files, pkg.Pkg)
	if err != nil {
		return nil, err
	}
	ct := closedutil.Find(t, closedTypes)
	if ct == nil {
		return nil, fmt.Errorf("%s is not recognized as a closed type", t.Name())
	}
	switch ct.(type) {
	default:
		return nil, fmt.Errorf("internal error: unrecognized closed type %T", ct)
	case *closed.Bitset, *closed.OptionalStruct:
		return nil, fmt.Errorf("%T cannot be used in switch", ct)
	case *closed.Enum, *closed.Interface, *closed.EmptySum:
		return ct, nil
	}
}

//shrinkUsed removes item i from a TypeAndValue slice.
func shrinkUsed(used []types.TypeAndValue, i int) []types.TypeAndValue {
	if len(used) == 0 {
		return nil
	}
	last := len(used) - 1
	used[last], used[i] = used[i], used[last]
	return used[:last]
}

//removeNilCase removes "case nil:" from a TypeAndValue slice and reports whether it had to.
func removeNilCase(ts []types.TypeAndValue) ([]types.TypeAndValue, bool) {
	for i, t := range ts {
		if t.IsNil() {
			return shrinkUsed(ts, i), true
		}
	}
	return ts, false
}

//usedBy returns the types used by by.
func usedBy(b *ast.BlockStmt, m map[ast.Expr]types.TypeAndValue) (acc []types.TypeAndValue, noDefault bool) {
	noDefault = true
	for _, c := range b.List {
		c := c.(*ast.CaseClause)
		if c.List == nil {
			noDefault = false
		}
		for _, x := range c.List {
			if tv, ok := m[x]; ok {
				acc = append(acc, tv)
			}
		}
	}
	return acc, noDefault
}
