package main

import (
	"go/constant"
	"go/token"
	"go/types"

	"github.com/jimmyfrasche/closed"
	"github.com/jimmyfrasche/closed/cmds/internal/closedutil"
)

func missingEnumCases(e *closed.Enum, used []types.TypeAndValue, diffPkgs bool) (acc [][]*types.Const, addZero bool, kind constant.Kind) {
	vs, hasZero := valuesOf(used)
	addZero = !hasZero && !e.NonZero && !closedutil.ContainsLabeledZero(e)

	ls := e.Labels
	kind = kindOf(ls)
	if diffPkgs {
		ls = exportedLabels(ls)
		if len(ls) == 0 {
			return nil, addZero, kind
		}
	}

	ls = unusedEnumCases(ls, vs)

	return ls, addZero, kind
}

func ceq(a, b constant.Value) bool {
	return constant.Compare(a, token.EQL, b)
}

func czero(k constant.Kind) constant.Value {
	var z constant.Value
	switch k {
	case constant.String:
		z = constant.MakeString("")
	case constant.Bool:
		z = constant.MakeBool(false)
	case constant.Int:
		z = constant.MakeInt64(0)
	case constant.Float:
		z = constant.MakeFloat64(0)
	case constant.Complex:
		z = constant.ToComplex(constant.MakeFloat64(0))
	default:
		z = constant.MakeUnknown()
	}
	return z
}

func valuesOf(tvs []types.TypeAndValue) (vs []constant.Value, hasZero bool) {
	if len(tvs) == 0 {
		return nil, false
	}

	z := czero(tvs[0].Value.Kind())
	for _, tv := range tvs {
		vs = append(vs, tv.Value)
		if ceq(tv.Value, z) {
			hasZero = true
		}
	}

	return vs, hasZero
}

func kindOf(ls [][]*types.Const) constant.Kind {
	return ls[0][0].Val().Kind()
}

func exportedLabels(ls [][]*types.Const) (acc [][]*types.Const) {
	for _, L := range ls {
		if closedutil.FirstExportedLabel(L) != nil {
			acc = append(acc, L)
		}
	}
	return acc
}

func unusedEnumCases(ls [][]*types.Const, vs []constant.Value) (acc [][]*types.Const) {
	for _, L := range ls {
		var isUsed bool
		vs, isUsed = constCaseIsUsed(L[0].Val(), vs)
		if !isUsed {
			acc = append(acc, L)
		}
	}
	return acc
}

func constCaseIsUsed(c constant.Value, vs []constant.Value) (rem []constant.Value, hit bool) {
	for i, v := range vs {
		if ceq(c, v) {
			last := len(vs) - 1
			vs[last], vs[i] = vs[i], vs[last]
			return vs[:last], true
		}
	}
	return vs, false
}
