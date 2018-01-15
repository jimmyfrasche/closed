package main

import (
	"go/types"

	"github.com/jimmyfrasche/closed"
	"github.com/jimmyfrasche/closed/cmds/internal/closedutil"
)

func missingInterfaceCases(i *closed.Interface, used []types.TypeAndValue, diffPkgs bool) (acc []types.Type, addNil bool) {
	ms := i.Members

	used, hasNil := removeNilCase(used)

	//addNil if there is not a nil present and i allows nil
	addNil = !hasNil && !i.NonNil

	if diffPkgs {
		//filter out types that cannot be accessed
		ms = exportedInterfaceMembers(ms)

		//nothing can be accessed so we're done
		if len(ms) == 0 {
			return nil, addNil
		}
	}

	ms = unusedInterfaceCases(ms, used)

	for _, m := range ms {
		//If there's an exported label, grab the first
		//(necessary if different package, harmless otherwise)
		//If not, we're in the same package so just use m.Type.
		T := closedutil.FirstExportedType(m)
		if T == nil {
			T = m.Type
		}

		acc = append(acc, T)
	}

	return acc, addNil
}

func exportedInterfaceMembers(ms []*closed.TypeNamesAndType) []*closed.TypeNamesAndType {
	var acc []*closed.TypeNamesAndType
	for _, m := range ms {
		if closedutil.FirstExportedTypeName(m.TypeName) != nil {
			acc = append(acc, m)
		}
	}
	return acc
}

func unusedInterfaceCases(ms []*closed.TypeNamesAndType, ts []types.TypeAndValue) []*closed.TypeNamesAndType {
	var acc []*closed.TypeNamesAndType
	for _, m := range ms {
		var isUsed bool
		ts, isUsed = interfaceCaseIsUsed(m, ts)
		if !isUsed {
			acc = append(acc, m)
		}
	}
	//note, len(used) > 0 possible here, but we're not a linter so not our problem
	return acc
}

func interfaceCaseIsUsed(m *closed.TypeNamesAndType, ts []types.TypeAndValue) (rem []types.TypeAndValue, hit bool) {
	for i, t := range ts {
		if types.Identical(m.Type, t.Type) {
			return shrinkUsed(ts, i), true
		}
	}
	return ts, false
}
