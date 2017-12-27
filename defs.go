package closed

import (
	"go/types"
)

//A Type is one of the closed types recognized by this package.
type Type interface {
	//Types returns the TypeNames for this Type.
	//Types()[0] is always the name of the defined type,
	//and Types()[1:] are aliases to that type definied in the same package.
	Types() []*types.TypeName
	closed()
}

type isType struct{}

func (isType) closed() {}

//Enum is a set of const labels with a defined type that is not a bitset.
type Enum struct {
	isType
	typs []*types.TypeName
	//Labels are the valid members of the enumeration.
	//If len(Labels[i]) > 1, then Labels[i][1:] are synonyms.
	//For example, given
	//	type Enum int
	//	const (
	//		A Enum = iota
	//		B
	//		C = B
	//	)
	//there would be two Labels.
	//The first would just be A and the second would be B and C.
	Labels [][]*types.Const
}

func (e *Enum) Types() []*types.TypeName {
	return e.typs
}

//A Bitset.
type Bitset struct {
	isType
	typs []*types.TypeName
	//Flags are the single bit labels in this Bitset.
	Flags [][]*types.Const
	//OrFlags are any multibit convienence labels.
	OrFlags [][]*types.Const
}

func (b *Bitset) Types() []*types.TypeName {
	return b.typs
}

//TypeNamesAndType records the TypeName of a defined type (always TypeName[0])
//and any aliases in the same package (TypeName[1:]).
//
//The Type field is either identical to TypeName.Type() or
//types.NewPointer(TypeName.Type()), whichever satisfies the interface
//that this value is associated with.
type TypeNamesAndType struct {
	TypeName []*types.TypeName
	Type     types.Type
}

//An Interface is an interface with at least one unexported method.
//It may or may not be a sum type.
//It may or may not be a closed type, but likely is.
type Interface struct {
	isType
	typs []*types.TypeName
	//Members are the types defined in the same package that satisfy this interface.
	Members []*TypeNamesAndType
}

func (i *Interface) Types() []*types.TypeName {
	return i.typs
}

//An InterfaceSum follows the conventional means of marking a sum type
//by an empty, nullary, unexported method tag.
type InterfaceSum struct {
	isType
	typs []*types.TypeName
	//Members are the types in the sum.
	Members []*TypeNamesAndType
	//FalseMembers are unexported zero-sized types with the appropriate tag
	//that are used to embed in the exported members.
	FalseMembers []*TypeNamesAndType
	//TagMethods is a list of the tag methods used by this sum.
	TagMethods []string
}

func (i *InterfaceSum) Types() []*types.TypeName {
	return i.typs
}

//EmptySum is a defined empty interface externally specified
//to contain only a finite number of types.
type EmptySum struct {
	isType
	typs []*types.TypeName
	//Nil is true if the nil value is legal.
	Nil bool
	//Members are valid types for this sum.
	Members []types.Type
}

func (e *EmptySum) Types() []*types.TypeName {
	return e.typs
}

//An OptionalStruct is a struct of the form
//	struct {
//		set bool
//		val T
//	}
//where the field val is only valid if set is true.
type OptionalStruct struct {
	isType
	typs         []*types.TypeName
	Discriminant *types.Var
	Field        *types.Var
}

func (o *OptionalStruct) Types() []*types.TypeName {
	return o.typs
}
