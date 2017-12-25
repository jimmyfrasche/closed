package closed

import (
	"go/token"
	"go/types"
	"sync"
)

type methodSetCache struct {
	mu *sync.Mutex
	m  map[*types.TypeName]methodSets
}

var cache = &methodSetCache{
	mu: new(sync.Mutex),
	m:  map[*types.TypeName]methodSets{},
}

func (m *methodSetCache) get(t *types.TypeName) methodSets {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.m[t]
}

func (m *methodSetCache) put(t *types.TypeName, ms methodSets) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.m[t] = ms
}

type name struct {
	pkg string
	nm  string
}

type duble struct {
	token.Pos
	in, out  *types.Tuple
	variadic bool
}

type methodSet map[name]duble

func computeMethodSet(t types.Type) methodSet {
	ms := methodSet{}
	m := types.NewMethodSet(t)
	for i := 0; i < m.Len(); i++ {
		s := m.At(i)
		o := s.Obj().(*types.Func)
		sig := o.Type().(*types.Signature)
		var pkg string
		if !o.Exported() {
			pkg = o.Pkg().String()
		}
		ms[name{
			pkg: pkg,
			nm:  o.Name(),
		}] = duble{
			Pos:      o.Pos(),
			in:       sig.Params(),
			out:      sig.Results(),
			variadic: sig.Variadic(),
		}
	}
	return ms
}

//HasUnexported returns true if at least one method is unexported.
func (m methodSet) HasUnexported(wrt string) bool {
	for n := range m {
		if n.pkg == wrt {
			return true
		}
	}
	return false
}

//Satisfies reports whether m satisfies i.
func (m methodSet) Satisfies(i methodSet) bool {
	for nm, db := range i {
		S, ok := m[nm]
		if !ok {
			return false
		}

		//they both have methods with the same name, check that they have the same signature
		if db.variadic != S.variadic {
			return false
		}
		if !tupEqual(db.in, S.in) || !tupEqual(db.out, S.out) {
			return false
		}
	}
	return true
}

func tupEqual(a, b *types.Tuple) bool {
	if a.Len() != b.Len() {
		return false
	}
	for i := 0; i < a.Len(); i++ {
		if !types.Identical(a.At(i).Type(), b.At(i).Type()) {
			return false
		}
	}
	return true
}

type methodSets struct {
	//ptrT always nil if method set of interface/ptr type
	T, ptrT methodSet
}

func methodSetsOf(t *types.TypeName) methodSets {
	if ms := cache.get(t); ms.T != nil {
		return ms
	}

	T := t.Type()

	var ms methodSets
	ms.T = computeMethodSet(T)

	switch T.(type) {
	case *types.Interface, *types.Pointer:
	default:
		ms.ptrT = computeMethodSet(types.NewPointer(T))
	}

	cache.put(t, ms)
	return ms
}
