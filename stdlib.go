package closed

import (
	"go/types"
	"strings"
)

//special cases for the standard library

func stdDatabaseSql(potOpts []*types.TypeName) []Type {
	var acc []Type
	for _, t := range potOpts {
		if !strings.HasPrefix(t.Name(), "Null") {
			continue
		}

		T := t.Type().Underlying().(*types.Struct)

		acc = append(acc, &OptionalStruct{
			typs:         []*types.TypeName{t},
			Discriminant: T.Field(1),
			Field:        T.Field(0),
		})
	}
	return acc
}

func stdEncodingXml(ts []*types.TypeName, s *types.Scope) []Type {
	out := make([]Type, 1)
	for _, t := range ts {
		if t.Name() != "Token" {
			continue
		}
		ms := []types.Type{
			s.Lookup("StartElement").Type(),
			s.Lookup("EndElement").Type(),
			s.Lookup("Comment").Type(),
			s.Lookup("ProcInst").Type(),
			s.Lookup("Directive").Type(),
		}
		out[0] = &EmptySum{
			typs:    []*types.TypeName{t},
			Nil:     false,
			Members: ms,
		}
	}
	return out
}

func stdEncodingJson(ts []*types.TypeName, s *types.Scope) []Type {
	out := make([]Type, 1)
	for _, t := range ts {
		if t.Name() != "Token" {
			continue
		}
		ms := []types.Type{
			s.Lookup("Delim").Type(),
			types.Universe.Lookup("bool").Type(),
			types.Universe.Lookup("float64").Type(),
			s.Lookup("Number").Type(),
			types.Universe.Lookup("string").Type(),
		}
		out[0] = &EmptySum{
			typs:    []*types.TypeName{t},
			Nil:     true,
			Members: ms,
		}
	}
	return out
}
