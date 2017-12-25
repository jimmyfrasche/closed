package closed

import (
	"go/ast"
	"go/token"
	"sort"
)

func tokenIn(x token.Token, ys ...token.Token) bool {
	for _, y := range ys {
		if x == y {
			return true
		}
	}
	return false
}

type decl struct {
	start, end token.Pos
	decl       *ast.FuncDecl
}

//declsInFile returns a sorted index of relevant decls or the first BadDecl.
func declsInFile(fs []*ast.File) (consts map[string]*ast.ValueSpec, decls []decl, err *ast.BadDecl) {
	consts = map[string]*ast.ValueSpec{}
	for _, f := range fs {
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.GenDecl:
				if d.Tok != token.CONST {
					continue
				}

				for _, spec := range d.Specs {
					vs := spec.(*ast.ValueSpec)
					for _, nm := range vs.Names {
						consts[nm.Name] = vs
					}
				}

			case *ast.FuncDecl:
				decls = append(decls, decl{
					start: d.Pos(),
					end:   d.End(),
					decl:  d,
				})

			case *ast.BadDecl:
				return nil, nil, d
			}
		}
	}

	sort.Slice(decls, func(i, j int) bool {
		return decls[i].start < decls[j].start
	})
	return consts, decls, nil
}

//declsFor returns the ast.Decl containing pos.
func declFor(decls []decl, pos token.Pos) *ast.FuncDecl {
	//even for a very large package len(decls) is going to be small
	//and on a well typed package a hit is guaranteed, and we're going
	//to call this at most once per func/const,
	//so linear search should be fine, but a better data structure
	//for efficient range queries wouldn't hurt.
	for _, d := range decls {
		if d.start <= pos && pos <= d.end {
			return d.decl
		}
	}
	return nil
}
