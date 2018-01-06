package closed

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"math/bits"
	"sort"
)

func grabEnums(fs *token.FileSet, decls map[string]*ast.ValueSpec, consts []*types.Const) (enums, bitsets map[*types.TypeName][]*constants, err error) {
	intvec, bools, restvec := binConstants(filterConstants(consts))

	enums = map[*types.TypeName][]*constants{}

	//bool constants are always enums, but have to be treated separately as the rest
	//depends on < being defined on the type
	for t, cs := range groupConstants(bools) {
		enums[t] = groupBool(cs)
	}

	//nonintegral types are always enums, by our definition
	for t, cs := range groupConstants(restvec) {
		enums[t] = groupLabels(cs)
	}

	ints, intbits, err := integralConsts(fs, decls, intvec)
	if err != nil {
		return nil, nil, err
	}

	for t, cs := range ints {
		//int enum with one member usually a sentinel
		if len(cs) > 1 {
			enums[t] = groupLabels(cs)
		}
	}
	ints = nil

	bitsets = map[*types.TypeName][]*constants{}
	for t, cs := range intbits {
		labels := groupLabels(cs)
		if len(labels) == 1 || !hasBitsetValues(labels) {
			continue
		}
		bitsets[t] = labels
	}

	return enums, bitsets, nil
}

func integralConsts(fs *token.FileSet, decls map[string]*ast.ValueSpec, consts []*types.Const) (enums, bitsets map[*types.TypeName][]*types.Const, err error) {
	specs, err := specsOfConsts(decls, consts)
	if err != nil {
		return nil, nil, err
	}

	groups := groupConstants(consts)
	bitsets = map[*types.TypeName][]*types.Const{}
	for t, cs := range groups {
		ok, err := allValidExprs(fs, specs, cs)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			bitsets[t] = cs
			delete(groups, t)
		}
	}

	return groups, bitsets, nil
}

//filerConstants that are of a named type defined in the same package.
func filterConstants(consts []*types.Const) []*types.Const {
	var acc []*types.Const
	for _, c := range consts {
		nm, ok := c.Type().(*types.Named)
		if !ok || c.Pkg() != nm.Obj().Pkg() {
			continue
		}
		acc = append(acc, c)
	}
	return acc
}

//binConstants into signed ints, unsigned ints, and the rest.
func binConstants(consts []*types.Const) (ints, bools, rest []*types.Const) {
	for _, c := range consts {
		k := c.Type().Underlying().(*types.Basic).Kind()
		switch k {
		case types.Int, types.Int8, types.Int16, types.Int32, types.Int64, types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64:
			ints = append(ints, c)
		case types.Bool:
			bools = append(bools, c)
		default:
			rest = append(rest, c)
		}
	}
	return
}

//groupConstants by their type.
func groupConstants(consts []*types.Const) map[*types.TypeName][]*types.Const {
	m := map[*types.TypeName][]*types.Const{}
	for _, c := range consts {
		t := c.Type().(*types.Named).Obj()
		m[t] = append(m[t], c)
	}
	return m
}

type constants struct {
	val    constant.Value
	labels []*types.Const
}

func groupBool(consts []*types.Const) []*constants {
	if len(consts) == 0 {
		return nil
	}

	var t, f []*types.Const
	T := constant.MakeBool(true)
	F := constant.MakeBool(false)

	for _, c := range consts {
		if constant.Compare(c.Val(), token.EQL, T) {
			t = append(t, c)
		} else {
			f = append(f, c)
		}
	}

	return []*constants{
		{val: F, labels: f},
		{val: T, labels: t},
	}
}

//groupLabels of a vector of constants of homogeneous type.
func groupLabels(consts []*types.Const) []*constants {
	if len(consts) == 0 {
		return nil
	}

	//sort incoming so all equal labels are in a row
	sort.Slice(consts, func(i, j int) bool {
		return constant.Compare(consts[i].Val(), token.LEQ, consts[j].Val())
	})

	var acc []*constants
	var last constant.Value
	for _, c := range consts {
		//no entries or a different value than the last
		if len(acc) == 0 || constant.Compare(c.Val(), token.NEQ, last) {
			acc = append(acc, &constants{
				val:    c.Val(),
				labels: []*types.Const{c},
			})
			last = c.Val()
			continue
		}

		//otherwise add to the current entries label set
		n := len(acc) - 1
		acc[n].labels = append(acc[n].labels, c)
	}

	return acc
}

//specsOfConsts associates typed constants with their ast spec.
func specsOfConsts(nms map[string]*ast.ValueSpec, consts []*types.Const) (map[*types.Const]*ast.ValueSpec, error) {
	m := map[*types.Const]*ast.ValueSpec{}
	for _, c := range consts {
		var ok bool
		nm := c.Name()
		m[c], ok = nms[nm]
		if !ok {
			return nil, fmt.Errorf("could not find ValueSpec for %q", nm)
		}
	}
	return m, nil
}

//TODO really need to replace this with finding a decl like 1 << iota

//allValidExprs uses spec to determine if consts (all of one type) only contains legal enum expressions.
func allValidExprs(fs *token.FileSet, spec map[*types.Const]*ast.ValueSpec, consts []*types.Const) (ok bool, err error) {
	defer func() {
		if x := recover(); x != nil {
			if expr, ok := x.(ast.Node); ok {
				p := fs.Position(expr.Pos())
				err = fmt.Errorf("%s: unexpected %T in const definition", p, expr)
			} else {
				panic(x)
			}
		}
	}()
	for _, c := range consts {
		s := spec[c]
		//part of an iota, so we only care about the iota line
		if len(s.Values) == 0 {
			continue
		}

		//find expression for constant
		var expr ast.Expr
		for i, ident := range s.Names {
			if ident.Name == c.Name() {
				expr = s.Values[i]
				break
			}
		}

		if !validExpr(expr) {
			return false, nil
		}
	}

	return true, nil
}

func validExpr(x ast.Expr) bool {
	switch x := x.(type) {
	default:
		panic(x)
	case *ast.Ident: // A = B
		return true
	case *ast.BasicLit: // A = 5
		return true
	case *ast.ParenExpr:
		return validExpr(x.X)
	case *ast.SelectorExpr: //A = pkg.A
		return true
	case *ast.CallExpr: // typeConversion(x)
		return validExpr(x.Args[0])
	case *ast.UnaryExpr:
		return validExpr(x.X)
	case *ast.BinaryExpr:
		//while there are other bitwise operators these are the ones
		//that always make it a bitset
		if tokenIn(x.Op, token.SHR, token.SHL, token.OR) {
			return false
		}
		return validExpr(x.X) && validExpr(x.Y)
	}
}

func allPositive(cs []*types.Const) bool {
	z := constant.MakeInt64(0)
	for _, c := range cs {
		if constant.Compare(c.Val(), token.LSS, z) {
			return false
		}
	}
	return true
}

func hasBitsetValues(lbls []*constants) bool {
	vals := make([]constant.Value, len(lbls))
	for i, lbl := range lbls {
		vals[i] = lbl.val
	}

	//partition into multibit values and tracking info for unibit values
	var all uint64
	unibit := 0
	multibit := make([]uint64, 0, len(vals))
	for _, v := range vals {
		//we're also checking signed ints, but they're always positive
		u, _ := constant.Uint64Val(v)
		if u == 0 {
			continue
		}

		if bits.OnesCount64(u) == 1 {
			all |= u
			unibit++
		} else {
			multibit = append(multibit, u)
		}
	}

	//too few unibit values to make decision confidently
	if unibit < 2 {
		return false
	}

	if len(multibit) > unibit/2 {
		return false
	}

	for _, u := range multibit {
		//reject if multibit value has bits not in any single bit value
		if u&^all != 0 {
			return false
		}
	}

	return true
}

func sortLabels(fs *token.FileSet, labels []*types.Const) {
	sort.Slice(labels, func(i, j int) bool {
		a, b := labels[i], labels[j]
		//if they're defined in different files, first sort by file name
		if fs.File(a.Pos()).Name() < fs.File(b.Pos()).Name() {
			return true
		}
		//otherwise rely on monotinicity of positions
		return a.Pos() < b.Pos()
	})
}

func pkgEnums(fs *token.FileSet, aliases map[string][]*types.TypeName, enums map[*types.TypeName][]*constants) []Type {
	acc := make([]Type, 0, len(enums))
	for t, cs := range enums {
		names := transClosureAliases(aliases, t)

		lbls := make([][]*types.Const, len(cs))
		for i, c := range cs {
			sortLabels(fs, c.labels)
			lbls[i] = c.labels
		}

		acc = append(acc, &Enum{
			typs:   names,
			Labels: lbls,
		})
	}
	return acc
}

func pkgBitsets(fs *token.FileSet, aliases map[string][]*types.TypeName, bitsets map[*types.TypeName][]*constants) []Type {
	acc := make([]Type, 0, len(bitsets))
	for t, cs := range bitsets {
		names := transClosureAliases(aliases, t)

		var flag, multibit [][]*types.Const
		for _, c := range cs {
			sortLabels(fs, c.labels)

			u, _ := constant.Uint64Val(c.val)
			if bits.OnesCount64(u) == 1 {
				flag = append(flag, c.labels)
			} else {
				multibit = append(multibit, c.labels)
			}
		}

		acc = append(acc, &Bitset{
			typs:    names,
			Flags:   flag,
			OrFlags: multibit,
		})
	}
	return acc
}
