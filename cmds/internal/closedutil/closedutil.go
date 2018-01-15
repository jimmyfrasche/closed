package closedutil

import (
	"fmt"
	"go/constant"
	"go/token"
	"go/types"

	"github.com/jimmyfrasche/closed"
)

//AlwaysValid returns true if t is always valid.
//
//This happens for bitsets with flags for each bit
//and an enum whose underlying type is bool.
func AlwaysValid(t closed.Type) bool {
	switch t := t.(type) {
	case *closed.Enum:
		k := t.Labels[0][0].Type().Underlying().(*types.Basic).Kind()
		return k == types.Bool
	case *closed.Bitset:
		k := t.Flags[0][0].Type().Underlying().(*types.Basic).Kind()
		L := len(t.Flags)
		switch k {
		case types.Uint8:
			return L == 8
		case types.Uint16:
			return L == 16
		case types.Uint32:
			return L == 32
		case types.Uint, types.Uint64:
			//int/uint only safe if falsely assumed to be 64bit
			return L == 64

		case types.Int8:
			return L == 7
		case types.Int16:
			return L == 17
		case types.Int32:
			return L == 31
		case types.Int, types.Int64:
			return L == 63

		default:
			return false
		}
	}
	return false
}

//ContainsLabeledZero reports whether t includes an explicit label for its zero value.
func ContainsLabeledZero(t *closed.Enum) bool {
	//bool enums contain false by definition
	k := t.Labels[0][0].Type().Underlying().(*types.Basic).Kind()
	if k == types.Bool {
		return true
	}
	var z constant.Value
	if k == types.String {
		z = constant.MakeString("")
	} else {
		//constant package will handle conversions for numeric types
		z = constant.MakeUint64(0)
	}
	for _, L := range t.Labels {
		if constant.Compare(L[0].Val(), token.EQL, z) {
			return true
		}
	}
	return false
}

//ExternallyExhaustible is true if every label/member/field of t is exported.
//
//The question of whether the type of t itself is exported is unrelated.
func ExternallyExhaustible(t closed.Type) bool {
	switch t := t.(type) {
	case *closed.Bitset:
		return eachHasExportedLabels(t.Flags)
	case *closed.Enum:
		return eachHasExportedLabels(t.Labels)

	case *closed.OptionalStruct:
		return t.Discriminant.Exported() && t.Field.Exported()

	case *closed.Interface:
		return eachHasExportedNames(t.Members)

	case *closed.EmptySum:
		return eachXR(t.Members)

	default:
		panic(fmt.Errorf("unexpected type %T", t))
	}
}

//eachXR tests whether each t is externally representable.
func eachXR(ts []types.Type) bool {
	for _, t := range ts {
		if !isXR(t) {
			return false
		}
	}
	return true
}

func isXR(t types.Type) bool {
	switch t := t.(type) {
	case *types.Basic:
		return true
	case *types.Named:
		return t.Obj().Exported()

	case *types.Pointer:
		return isXR(t.Elem())
	case *types.Array:
		return isXR(t.Elem())
	case *types.Slice:
		return isXR(t.Elem())
	case *types.Chan:
		return isXR(t.Elem())
	case *types.Map:
		return isXR(t.Key()) && isXR(t.Elem())

	case *types.Signature:
		return tupleXR(t.Params()) && tupleXR(t.Results())

	case *types.Struct:
		return structXR(t)

	case *types.Interface:
		return interfaceXR(t)
	default:
		panic(fmt.Errorf("unexpected type %T", t))
	}
}

func tupleXR(t *types.Tuple) bool {
	for i := 0; i < t.Len(); i++ {
		if !isXR(t.At(i).Type()) {
			return false
		}
	}
	return true
}

func structXR(t *types.Struct) bool {
	for i := 0; i < t.NumFields(); i++ {
		f := t.Field(i)
		//In this context it's a literal struct so the field names
		//must be exported, too.
		if !f.Exported() || !isXR(f.Type()) {
			return false
		}
	}
	return true
}

func interfaceXR(t *types.Interface) bool {
	for i := 0; i < t.NumMethods(); i++ {
		m := t.Method(i)
		//Since we're concerned with interfaces in interfaces here,
		//we don't care about the identity: just the method set.
		if !m.Exported() || !isXR(m.Type()) {
			return false
		}
	}
	return true
}

func eachHasExportedLabels(vss [][]*types.Const) bool {
	for _, vs := range vss {
		if FirstExportedLabel(vs) == nil {
			return false
		}
	}
	return true
}

//FirstExportedLabel returns the first v in vs with an exported name
//or nil if there is none.
func FirstExportedLabel(vs []*types.Const) *types.Const {
	for _, v := range vs {
		if v.Exported() {
			return v
		}
	}
	return nil
}

func eachHasExportedNames(ms []*closed.TypeNamesAndType) bool {
	for _, m := range ms {
		if FirstExportedTypeName(m.TypeName) == nil {
			return false
		}
	}
	return true
}

//FirstExportedTypeName returns the first t in ts with an exported name
//or nil if there are none.
func FirstExportedTypeName(ts []*types.TypeName) *types.TypeName {
	for _, n := range ts {
		if n.Exported() {
			return n
		}
	}
	return nil
}

//FirstExportedType returns a Type correponding to FirstExportedTypeName
//which is a pointer if t.Type is a pointer.
func FirstExportedType(t *closed.TypeNamesAndType) types.Type {
	T := FirstExportedTypeName(t.TypeName)
	if T == nil {
		return nil
	}
	nm := T.Type()
	_, isPtr := t.Type.(*types.Pointer)
	if isPtr {
		return types.NewPointer(nm)
	}
	return nm
}

//AllMask returns a mask with every bit in b.Flags set.
func AllMask(b *closed.Bitset) uint64 {
	var all uint64
	for _, f := range b.Flags {
		v, _ := constant.Uint64Val(f[0].Val())
		all |= v
	}
	return all
}

type pkger = interface {
	Pkg() *types.Package
}

//ImportsOf extracts all packages that need to be imported in a file
//exhaustively testing members of c.
func ImportsOf(c closed.Type) (imports map[string]bool, err error) {
	imports = map[string]bool{}
	imp := func(p pkger) {
		imports[p.Pkg().Path()] = true
	}

	defer func() {
		if x := recover(); x != nil {
			if e, ok := x.(error); ok {
				err = e
				return
			}
			panic(x)
		}
	}()

	imp(c.Types()[0])

	switch c := c.(type) {
	case *closed.OptionalStruct:
		importsOf(imp, c.Field.Type())

	case *closed.EmptySum:
		for _, m := range c.Members {
			importsOf(imp, m)
		}
	case *closed.Bitset, *closed.Enum, *closed.Interface:
		//already done
	default:
		return nil, fmt.Errorf("closedutil.ImportsOf: unexpected %T", c)
	}

	return imports, nil
}

func importsOf(imp func(pkger), t types.Type) {
	switch t := t.(type) {
	case *types.Basic:
		//nothing to do

	case *types.Pointer:
		importsOf(imp, t.Elem())
	case *types.Array:
		importsOf(imp, t.Elem())
	case *types.Slice:
		importsOf(imp, t.Elem())
	case *types.Chan:
		importsOf(imp, t.Elem())
	case *types.Map:
		importsOf(imp, t.Key())
		importsOf(imp, t.Elem())

	case *types.Struct:
		for i := 0; i < t.NumFields(); i++ {
			importsOf(imp, t.Field(i).Type())
		}
	case *types.Interface:
		for i := 0; i < t.NumEmbeddeds(); i++ {
			imp(t.Embedded(i).Obj())
		}
		for i := 0; i < t.NumExplicitMethods(); i++ {
			importsOf(imp, t.ExplicitMethod(i).Type())
		}

	case *types.Signature:
		importsOf(imp, t.Params())
		importsOf(imp, t.Results())

	case *types.Tuple:
		for i := 0; i < t.Len(); i++ {
			importsOf(imp, t.At(i).Type())
		}

	case *types.Named:
		imp(t.Obj())
	}
}

//Find t in vs or return nil.
func Find(t *types.TypeName, vs []closed.Type) closed.Type {
	for _, v := range vs {
		for _, n := range v.Types() {
			if t == n {
				return v
			}
		}
	}
	return nil
}
