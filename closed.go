package closed

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
)

//InPackage extracts closed types from a given package.
//
//All parameters are required.
//
//The pkg is expected to be error free.
//If not, the results of InPackage may be incorrect or incomplete.
//
//The files must be all the files used to parse pkg.
//
//The fs must be the FileSet used to parse pkg.
func InPackage(fs *token.FileSet, files []*ast.File, pkg *types.Package) ([]Type, error) {
	constDecls, funcDecls, bad := declsInFile(files)
	if bad != nil {
		return nil, fmt.Errorf("%s: bad declaration", fs.Position(bad.Pos()))
	}

	consts, allTypes := extract(pkg.Scope())
	aliases, regTypes := findAliasesAndRegular(allTypes)

	enums, err := grabEnums(fs, constDecls, consts)
	if err != nil {
		return nil, err
	}

	out := pkgEnums(fs, aliases, enums)

	abstract, concrete := split(regTypes)

	potOpts, _ := potentiallyClosedStructs(concrete)

	if pkg.Path() == "database/sql" {
		out = append(out, stdDatabaseSql(potOpts)...)
	}

	empties, closed := binInterfaces(abstract)

	if pkg.Path() == "encoding/xml" {
		out = append(out, stdEncodingXml(empties, pkg.Scope())...)
	}
	if pkg.Path() == "encoding/json" {
		out = append(out, stdEncodingJson(empties, pkg.Scope())...)
	}

	sats := satisfiers(closed, concrete)

	ifaces := pkgIfaces(aliases, funcDecls, sats)
	out = append(out, ifaces...)

	return out, nil
}

//extract what we care about from scope.
func extract(s *types.Scope) (consts []*types.Const, allTypes []*types.TypeName) {
	for _, nm := range s.Names() {
		switch o := s.Lookup(nm).(type) {
		case *types.Const:
			consts = append(consts, o)
		case *types.TypeName:
			allTypes = append(allTypes, o)
		default:
			//discard
		}
	}
	return
}
