package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"strconv"
)

//switchesOf finds all switches and type switches in f.
func switchesOf(f *ast.File) []ast.Stmt {
	var switches []ast.Stmt
	for _, d := range f.Decls {
		ast.Inspect(d, func(n ast.Node) bool {
			if s, ok := n.(*ast.SwitchStmt); ok {
				//note that we could check if s.Tag == nil here,
				//but we can produce error messages if we defer checking.
				switches = append(switches, s)
			} else if s, ok := n.(*ast.TypeSwitchStmt); ok {
				switches = append(switches, s)
			}
			return true
		})
	}
	return switches
}

//findSwitch finds the switch in switches that has the tightest matching to the line/offset requested.
func findSwitch(fs *token.FileSet, astf *ast.File, line, offset int, switches []ast.Stmt) (ast.Stmt, error) {
	//we just need to configure the switchFinder and check for egregious errors.
	tf := fs.File(astf.Pos())
	fname := filepath.Base(tf.Name())

	finder := &switchFinder{
		fname:    filepath.Base(tf.Name()),
		switches: switches,
	}

	if line > 0 { // only one of line, offset > 0 by construction
		if lc := tf.LineCount(); lc < line {
			return nil, fmt.Errorf("impossible line number %d: only %d lines in %s", line, lc, fname)
		}

		finder.n = line
		finder.kind = "line"
		finder.extract = func(p token.Pos) int {
			return fs.Position(p).Line
		}
	} else {
		if sz := tf.Size(); sz < offset {
			return nil, fmt.Errorf("impossible offset %d: only %d bytes in %s", offset, sz, fname)
		}

		finder.n = offset
		finder.kind = "offset"
		finder.extract = func(p token.Pos) int {
			return fs.Position(p).Offset
		}
	}

	return finder.find()
}

type switchFinder struct {
	fname, kind string
	switches    []ast.Stmt
	extract     func(t token.Pos) int // looks up Offset or Line
	n           int
}

func (f *switchFinder) find() (ast.Stmt, error) {
	min := int(^uint(0) >> 1) //init with largest int
	cands := map[int]ast.Stmt{}

	for _, s := range f.switches {
		start := f.extract(s.Pos())
		end := f.extract(s.End())

		//found a switch statement that contains f.n,
		//but switches can nest so we need to find
		//all such switches
		if start <= f.n && f.n <= end {
			//since all candidates contain f.n
			//the one we're looking for is the one
			//with the minimum distance between its
			//start and end, which will be cands[min].
			d := end - start
			if d < min {
				min = d
			}

			if _, ok := cands[d]; ok {
				//two switches with same distance
				//this can only happen in line mode
				//if the input contains two switches on the same line
				//so the user needs to run gofmt
				return nil, fmt.Errorf("multiple switches match on line %d in %s, run gofmt or specify offset", f.n, f.fname)
			}

			cands[d] = s
		}
	}

	if len(cands) == 0 {
		return nil, fmt.Errorf("no switch statement found at %s in %s", f.kind, f.fname)
	}

	return cands[min], nil
}

//importsOfFile returns the imports of f as a map
//from the import path to the local name (or "" for the default).
func importsOfFile(f *ast.File) map[string]string {
	m := make(map[string]string, len(f.Imports))
	for _, is := range f.Imports {
		var nm string
		if is.Name != nil {
			nm = is.Name.Name
		}

		p, err := strconv.Unquote(is.Path.Value)
		if err != nil {
			//this will never happen so might as well explode if it somehow does
			panic(err)
		}

		m[p] = nm
	}
	return m
}

type typeSerializer struct {
	buf  bytes.Buffer
	pkg  *types.Package
	imps importMap
}

func newTypeSerializer(in *types.Package, imps importMap) *typeSerializer {
	return &typeSerializer{
		pkg:  in,
		imps: imps,
	}
}

func (p *typeSerializer) qualify(pkg *types.Package) string {
	if p.pkg == pkg {
		return ""
	}
	return p.imps.Name(pkg)
}

func (p *typeSerializer) print(t types.Type) (ast.Expr, error) {
	p.buf.Reset()
	types.WriteType(&p.buf, t, p.qualify)
	return parser.ParseExpr(p.buf.String())
}

func body(sw ast.Stmt) (block *ast.BlockStmt, isTypeSwitch bool) {
	switch sw := sw.(type) {
	case *ast.SwitchStmt:
		return sw.Body, false
	case *ast.TypeSwitchStmt:
		return sw.Body, true
	}
	panic("unreachable")
}

func mkNil() ast.Expr {
	return ast.NewIdent("nil")
}

func mkZero(k constant.Kind) ast.Expr {
	switch k {
	case constant.Bool:
		return ast.NewIdent("false")

	case constant.String:
		return &ast.BasicLit{
			Kind:  token.STRING,
			Value: `""`,
		}

	default: //numeric
		return &ast.BasicLit{
			Kind:  token.INT,
			Value: "0",
		}
	}
}

func mkDefault() *ast.CaseClause {
	return &ast.CaseClause{}
}

func mkLabel(name, pkg string) ast.Expr {
	if pkg == "" {
		return ast.NewIdent(name)
	}

	return &ast.SelectorExpr{
		X:   ast.NewIdent(pkg),
		Sel: ast.NewIdent(name),
	}
}

func mkCase(xs ...ast.Expr) *ast.CaseClause {
	return &ast.CaseClause{
		List: xs,
	}
}

func toCaseClauses(xs []ast.Expr, flat bool) []ast.Stmt {
	if flat {
		return []ast.Stmt{
			mkCase(xs...),
		}
	}

	cs := make([]ast.Stmt, 0, len(xs))
	for _, x := range xs {
		cs = append(cs, mkCase(x))
	}
	return cs
}

func spliceClauses(sw ast.Stmt, cases []ast.Stmt, defaultCase *ast.CaseClause) ast.Stmt {
	if len(cases) == 0 && defaultCase == nil {
		return sw
	}
	switch sw := sw.(type) {
	case *ast.SwitchStmt:
		sw.Body.List = addBlock(sw.Body.List, cases, defaultCase)
	case *ast.TypeSwitchStmt:
		sw.Body.List = addBlock(sw.Body.List, cases, defaultCase)
	}
	return sw
}

func addBlock(block []ast.Stmt, cases []ast.Stmt, defaultCase *ast.CaseClause) []ast.Stmt {
	//no existing cases to worry about
	if len(block) == 0 {
		if defaultCase != nil {
			cases = append(cases, defaultCase)
		}
		return cases
	}

	//If the first clause is a default, insert after it
	insertAt := 0
	if len(block) > 0 {
		if c, ok := block[0].(*ast.CaseClause); ok && c.List == nil {
			insertAt = 1
		}
	}

	out := append([]ast.Stmt{}, block[:insertAt]...)
	after := block[insertAt:]

	out = append(out, cases...)
	out = append(out, after...)
	if defaultCase != nil {
		out = append(out, defaultCase)
	}

	return out
}
