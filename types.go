package closed

import (
	"go/types"
	"sort"
)

//interfacesAndConcrete into interfaces and non-interfaces.
func interfacesAndConcrete(ts []*types.TypeName) (abstract, concrete []*types.TypeName) {
	for _, t := range ts {
		if types.IsInterface(t.Type()) {
			abstract = append(abstract, t)
		} else {
			concrete = append(concrete, t)
		}
	}
	return
}

//findAliasesAndRegular collects all aliases for local types and all regular types
func findAliasesAndRegular(ts []*types.TypeName) (map[string][]*types.TypeName, []*types.TypeName) {
	alias := map[string][]*types.TypeName{}
	var real []*types.TypeName
	for _, t := range ts {
		if !t.IsAlias() {
			real = append(real, t)
			continue
		}

		nm, ok := t.Type().(*types.Named)
		if !ok {
			continue
		}
		o := nm.Obj()
		if o.Pkg() != t.Pkg() {
			continue
		}

		alias[o.Name()] = append(alias[o.Name()], t)
	}
	return alias, real
}

//potentiallyClosedStructs returns structs with at least two fields where the first is integral or boolean.
func potentiallyClosedStructs(ts []*types.TypeName) (optionals, unions []*types.TypeName) {
outer:
	for _, t := range ts {
		s, ok := t.Type().Underlying().(*types.Struct)
		if !ok || s.NumFields() < 2 {
			continue
		}

		//Two fields and one is a bool, may be optional type
		if s.NumFields() == 2 {
			for i := 0; i < 2; i++ {
				if f, ok := s.Field(i).Type().(*types.Basic); ok && f.Kind() == types.Bool {
					optionals = append(optionals, t)
					continue outer
				}
			}
		}

		//Otherwise, we insist that a discriminant is an enum in the first field
		f := s.Field(0).Type()
		if _, ok := f.(*types.Named); !ok {
			continue
		}

		B, ok := f.Underlying().(*types.Basic)
		if !ok {
			continue
		}

		if B.Kind() == types.Bool {
			if s.NumFields() == 2 {
				optionals = append(optionals, t)
			} else if s.NumFields() == 3 {
				unions = append(unions, t)
			}
			continue
		}

		//TODO need to confirm discriminant is a union
		switch B.Kind() {
		case types.Int, types.Int8, types.Int16, types.Int32, types.Int64, types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64:
			unions = append(unions, t)
		}
	}
	return optionals, unions
}

//transClosureAliases finds all local aliases of t.
func transClosureAliases(aliases map[string][]*types.TypeName, t *types.TypeName) []*types.TypeName {
	acc := transClosureAliasesRec(aliases, t, nil)
	//want to leave first element alone but keep rest in stable order
	sort.Slice(acc[1:], func(i, j int) bool {
		return acc[i+1].Name() < acc[j+1].Name()
	})
	return acc
}

func transClosureAliasesRec(aliases map[string][]*types.TypeName, t *types.TypeName, acc []*types.TypeName) []*types.TypeName {
	acc = append(acc, t)
	for _, alias := range aliases[t.Name()] {
		acc = transClosureAliasesRec(aliases, alias, acc)
	}
	return acc
}

//binInterfaces into defined empty ifaces and ifaces with at least one unexported
//method from the same package as its definition.
func binInterfaces(abstract []*types.TypeName) (empty, closed []*types.TypeName) {
	for _, i := range abstract {
		ims := methodSetsOf(i).T
		if len(ims) == 0 {
			empty = append(empty, i)
		} else if ims.HasUnexported(i.Pkg().String()) {
			closed = append(closed, i)
		}
	}
	return
}

type typeOrPtr struct {
	isPtr    bool
	TypeName *types.TypeName
}

func (t typeOrPtr) Type() types.Type {
	T := t.TypeName.Type()
	if t.isPtr {
		return types.NewPointer(T)
	}
	return T
}

//satisfiers of abstract among concrete.
func satisfiers(abstract []*types.TypeName, concrete []*types.TypeName) map[*types.TypeName][]typeOrPtr {
	m := map[*types.TypeName][]typeOrPtr{}
	for _, i := range abstract {
		ims := methodSetsOf(i).T
		for _, c := range concrete {
			ms := methodSetsOf(c)
			if ms.T.Satisfies(ims) {
				m[i] = append(m[i], typeOrPtr{TypeName: c})
			} else if ms.ptrT != nil && ms.ptrT.Satisfies(ims) {
				m[i] = append(m[i], typeOrPtr{
					isPtr:    true,
					TypeName: c,
				})
			}
		}
	}
	return m
}

//findTagMethods searches members for nullary unexported methods defined on sum
//that have empty bodies in all members and returns that subset
//(likely len 0 or 1)
func findTagMethods(decls []decl, sum *types.TypeName, members []typeOrPtr) methodSet {
	cands := methodSet{}
	for n, sig := range methodSetsOf(sum).T {
		if n.pkg != sum.Pkg().String() {
			continue
		}
		if sig.in.Len() != 0 || sig.out.Len() != 0 {
			continue
		}
		cands[n] = sig
	}
	if len(cands) == 0 {
		return nil
	}

	for _, m := range members {
		ms := methodSetsOf(m.TypeName)
		s := ms.T
		if m.isPtr {
			s = ms.ptrT
		}

		for n := range cands {
			d := declFor(decls, s[n].Pos)
			b := d.Body

			if b == nil || len(b.List) != 0 {
				delete(cands, n)
				continue
			}
		}
	}

	return cands
}

//pkgTagMethods formats a method set as a sorted []string of method names.
func pkgTagMethods(tags methodSet) []string {
	tagNames := make([]string, 0, len(tags))
	for nm := range tags {
		tagNames = append(tagNames, nm.nm)
	}
	sort.Strings(tagNames)
	return tagNames
}

//anyExported names in ts.
func anyExported(ts []*types.TypeName) bool {
	for _, t := range ts {
		if t.Exported() {
			return true
		}
	}
	return false
}

//sizer is only used to test for zero sized types so arch is irrelevant.
var sizer = types.SizesFor("gc", "amd64")

func zeroSized(t types.Type) bool {
	return sizer.Sizeof(t) == 0
}

//pkgIfaces packages interfaces into Interface and InterfaceSum.
func pkgIfaces(aliases map[string][]*types.TypeName, decls []decl, ifaces map[*types.TypeName][]typeOrPtr) []Type {
	acc := make([]Type, 0, len(ifaces))
	for t, ms := range ifaces {
		names := transClosureAliases(aliases, t)

		tags := findTagMethods(decls, t, ms)
		if len(tags) == 0 {
			acc = append(acc, pkgClosedInterface(aliases, names, t, ms))
		} else {
			acc = append(acc, pkgInterfaceSum(aliases, names, t, ms, tags))
		}
	}
	return acc
}

func pkgM(aliases map[string][]*types.TypeName, m typeOrPtr) *TypeNamesAndType {
	return &TypeNamesAndType{
		TypeName: transClosureAliases(aliases, m.TypeName),
		Type:     m.Type(),
	}
}

func pkgClosedInterface(aliases map[string][]*types.TypeName, names []*types.TypeName, t *types.TypeName, ms []typeOrPtr) *Interface {
	typs := make([]*TypeNamesAndType, len(ms))
	for i, m := range ms {
		typs[i] = pkgM(aliases, m)
	}
	return &Interface{
		typs:    names,
		Members: typs,
	}
}

func pkgInterfaceSum(aliases map[string][]*types.TypeName, names []*types.TypeName, t *types.TypeName, ms []typeOrPtr, tags methodSet) *Interface {
	real, fake := splitIfaceSumMethods(aliases, names, t, ms, tags)
	tagNames := pkgTagMethods(tags)
	return &Interface{
		typs:         names,
		Members:      real,
		FalseMembers: fake,
		TagMethods:   tagNames,
	}
}

func splitIfaceSumMethods(aliases map[string][]*types.TypeName, names []*types.TypeName, t *types.TypeName, ms []typeOrPtr, tags methodSet) (real, fake []*TypeNamesAndType) {
	checkFalse := mustCheckForFalseMembers(names, t, tags)
	var embeddings map[*types.TypeName][]types.Type

	typs := make([]*TypeNamesAndType, 0, len(ms))
	var falseTyps []*TypeNamesAndType

	for _, m := range ms {
		T := pkgM(aliases, m)

		if checkFalse && maybeFalse(T, tags) && isEmbedded(embeddings, T, ms) {
			falseTyps = append(falseTyps, T)
		} else {
			typs = append(typs, T)
		}
	}
	return typs, falseTyps
}

func mustCheckForFalseMembers(names []*types.TypeName, t *types.TypeName, tags methodSet) bool {
	return anyExported(names) && len(methodSetsOf(t).T) == len(tags)
}

func maybeFalse(T *TypeNamesAndType, tags methodSet) bool {
	if zeroSized(T.TypeName[0].Type()) && !anyExported(T.TypeName) {
		methods := methodSetsOf(T.TypeName[0])
		if lt := len(tags); len(methods.T) == lt || len(methods.ptrT) == lt {
			return true
		}
	}
	return false
}

func lazyComputeEmbeddingsOfMembers(em map[*types.TypeName][]types.Type, ms []typeOrPtr) map[*types.TypeName][]types.Type {
	//len(em) == 0 means we computed and found nothing
	if em != nil {
		return em
	}

	for _, m := range ms {
		T := m.TypeName

		s, ok := T.Type().(*types.Struct)
		if !ok || zeroSized(s) {
			continue
		}

		for i := 0; i < s.NumFields(); i++ {
			f := s.Field(i)
			if !f.Anonymous() {
				continue
			}
			em[T] = append(em[T], f.Type())
		}
	}

	return em
}

func isEmbedded(ems map[*types.TypeName][]types.Type, t *TypeNamesAndType, ms []typeOrPtr) bool {
	ems = lazyComputeEmbeddingsOfMembers(ems, ms)
	for _, ets := range ems {
		for _, ef := range ets {
			if types.Identical(t.Type, ef) {
				return true
			}
		}
	}
	return false
}
