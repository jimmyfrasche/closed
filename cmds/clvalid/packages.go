package main

import (
	"fmt"
	"sort"

	"github.com/jimmyfrasche/closed"
	"github.com/jimmyfrasche/closed/cmds/internal/closedutil"
	"github.com/jimmyfrasche/closed/cmds/internal/tools"
)

type Package struct {
	Name, ImportPath string
}

func newPackage(tags []string, imp string) (pkg *Package, err error) {
	var args []string
	if imp != "" {
		args = []string{imp}
	}
	imps, err := tools.GoList(tags, args)
	if err != nil {
		return nil, err
	}
	if len(imps) != 1 {
		return nil, fmt.Errorf("found %d package, need one", len(imps))
	}
	name, err := tools.PackageName(tags, imps[0])
	if err != nil {
		return nil, err
	}
	return &Package{
		Name:       name,
		ImportPath: imps[0],
	}, nil
}

func packages(buildTags []string, args []string) (typeName string, forPkg, fromPkg *Package, err error) {
	failWith := func(err error) (string, *Package, *Package, error) {
		return "", nil, nil, err
	}

	forPkg, err = newPackage(buildTags, "")
	if err != nil {
		return failWith(err)
	}

	//only dealing with one package
	if len(args) == 1 {
		return args[0], forPkg, forPkg, nil
	}

	//The current package was explicitly specified.
	//Treat as an error to keep go generate directives clean and uniform.
	if args[1] == forPkg.ImportPath {
		return failWith(fmt.Errorf("cannot specify import path %q when it is the current package", args[1]))
	}

	fromPkg, err = newPackage(buildTags, args[0])
	if err != nil {
		return failWith(err)
	}

	return args[1], forPkg, fromPkg, nil
}

//computeImports computes a stable set of local aliases
//for importing into the generated code and a map of import paths
//to these local aliases.
func computeImports(here string, c closed.Type) (imports []string, import2name map[string]string, err error) {
	impset, err := closedutil.ImportsOf(c)
	if err != nil {
		return nil, nil, err
	}

	sorted := make([]string, 0, len(impset))
	for k := range impset {
		if k == here {
			continue
		}
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)

	out := make(map[string]string, len(sorted))
	for i, v := range sorted {
		out[v] = fmt.Sprintf("pkg%d", i)
		imports = append(imports, fmt.Sprintf("pkg%d %q", i, v))
	}
	return imports, out, nil
}
