package main

import (
	"go/types"

	"github.com/jimmyfrasche/closed"
)

func missingEmptyCases(e *closed.EmptySum, used []types.TypeAndValue, pkg *types.Package) (acc []types.Type, addNil bool) {
	used, hasNil := removeNilCase(used)

	addNil = !hasNil && e.Nil

	ms := e.Members
	if e.Types()[0].Pkg() != pkg {
		ms = exportedTypesWRT(ms, pkg)
		if len(ms) == 0 {
			return ms, addNil
		}
	}

	ms = unusedEmptyCases(ms, used)

	return ms, addNil
}

func exportedTypesWRT(ts []types.Type, pkg *types.Package) []types.Type {
	var acc []types.Type
	for _, t := range ts {
		if representable(t, pkg) {
			acc = append(acc, t)
		}
	}
	return acc
}

func elem(t types.Type) types.Type {
	type elemer = interface {
		Elem() types.Type
	}
	return t.(elemer).Elem()
}

func representable(t types.Type, from *types.Package) bool {
	switch t := t.(type) {
	case *types.Basic:
		return true
	case *types.Named:
		o := t.Obj()
		return o.Exported() || o.Pkg() == from

	case *types.Pointer, *types.Array, *types.Slice, *types.Chan:
		return representable(elem(t), from)
	case *types.Map:
		return representable(t.Key(), from) && representable(t.Elem(), from)

	case *types.Struct:
		for i := 0; i < t.NumFields(); i++ {
			f := t.Field(i)
			if !f.Exported() || !representable(f.Type(), from) {
				return false
			}
		}
		return true

	case *types.Interface:
		for i := 0; i < t.NumEmbeddeds(); i++ {
			if !representable(t.Embedded(i).Underlying(), from) {
				return false
			}
		}
		for i := 0; i < t.NumExplicitMethods(); i++ {
			m := t.ExplicitMethod(i)
			if !m.Exported() && !representable(m.Type(), from) {
				return false
			}
		}
		return true

	case *types.Signature:
		return representable(t.Params(), from) && representable(t.Results(), from)
	case *types.Tuple:
		for i := 0; i < t.Len(); i++ {
			if !representable(t.At(i).Type(), from) {
				return false
			}
		}
		return true
	}
	panic("unreachable")
}

func unusedEmptyCases(ms []types.Type, used []types.TypeAndValue) (acc []types.Type) {
	for _, m := range ms {
		var isUsed bool
		used, isUsed = emptyCaseIsUsed(m, used)
		if !isUsed {
			acc = append(acc, m)
		}
	}
	return acc
}

func emptyCaseIsUsed(e types.Type, ts []types.TypeAndValue) (rem []types.TypeAndValue, hit bool) {
	for i, t := range ts {
		if types.Identical(e, t.Type) {
			return shrinkUsed(ts, i), true
		}
	}
	return ts, false
}
