//Command fillswitch(1) populates missing cases in Go switches
//when the expression switched is a *closed.Enum, *closed.Interface,
//or *closed.EmptySum.
//
//It is intended to be integrated into an editor.
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/format"
	"go/token"
	"go/types"
	"log"
	"os"

	"github.com/jimmyfrasche/closed"
	"github.com/jimmyfrasche/closed/cmds/fillswitch/internal/guess"
	"github.com/jimmyfrasche/closed/cmds/internal/closedutil"
	"github.com/jimmyfrasche/closed/cmds/internal/tools"
	"golang.org/x/tools/go/buildutil"
	"golang.org/x/tools/go/loader"
)

func failOn(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	log.SetPrefix(fmt.Sprintf("%s: ", os.Args[0]))
	log.SetFlags(0)

	tools.AddTagsFlagDefault()
	var (
		modified, save, flat bool
		offset, line         int
		imp                  string
	)
	flag.BoolVar(&modified, "modified", false, "read `archive` of modified files from stdin")
	flag.BoolVar(&save, "w", false, "write result to file, instead of stdout")
	flag.BoolVar(&flat, "flat", false, "output as a single case")
	flag.IntVar(&offset, "offset", 0, "byte offset of `cursor position` inside switch statement")
	flag.IntVar(&line, "line", 0, "line number inside switch statement")
	flag.StringVar(&imp, "import", "", "import path of package containing -file")

	flag.Usage = func() {
		log.SetPrefix("")
		log.Printf("usage: %s [flags] file", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		log.Print("requires exactly one filename as argument")
		flag.Usage()
		os.Exit(2)
	}
	file := flag.Arg(0)

	if offset < 0 {
		log.Fatalf("invalid offset %d, must be positive", offset)
	}
	if line < 0 {
		log.Fatalf("invalid line %d, must be positive", line)
	}
	if offset == 0 && line == 0 {
		log.Print("one of -offset or -line is required")
		flag.Usage()
		os.Exit(2)
	}
	if offset != 0 && line != 0 {
		log.Fatal("cannot specify both -offset and -line")
	}

	bc := &build.Default
	if modified {
		overlay, err := buildutil.ParseOverlayArchive(os.Stdin)
		failOn(err)

		if len(overlay) > 0 {
			bc = buildutil.OverlayContext(bc, overlay)
		}
	}

	if imp == "" {
		var err error
		_, imp, err = guess.ImportPath(file, bc)
		failOn(err)
	}

	fs, astf, sw, pkg, dpkg, prog, ct, err := extractSwitchAndTypeInfo(bc, imp, file, line, offset)
	failOn(err)

	//we need to do this even if no imports are added in order to find
	//out what the local names of packages are in astf
	//as there may be local aliases
	imps, err := addImportsAndGetLocalPackageNames(fs, astf, ct, pkg, dpkg, sw, prog)
	failOn(err)

	cases, defaultCase, err := computeCasesToAdd(sw, ct, pkg, dpkg, imps)
	failOn(err)

	clauses := toCaseClauses(cases, flat)
	sw = spliceClauses(sw, clauses, defaultCase)

	format.Node(os.Stdout, fs, astf)
}

func extractSwitchAndTypeInfo(bc *build.Context, imp, file string, line, offset int) (fs *token.FileSet, f *ast.File, sw ast.Stmt, cpkg, dpkg *loader.PackageInfo, prog *loader.Program, ct closed.Type, err error) {
	fail := func(err error) (fs *token.FileSet, f *ast.File, sw ast.Stmt, ckpg, dpkg *loader.PackageInfo, prog *loader.Program, ct closed.Type, e error) {
		return nil, nil, nil, nil, nil, nil, nil, err
	}

	prog, err = load(bc, imp)
	if err != nil {
		return fail(err)
	}

	pkg, astf, err := pkgWithFile(imp, file, prog)
	if err != nil {
		return fail(err)
	}

	switches := switchesOf(astf)
	if len(switches) == 0 {
		return fail(fmt.Errorf("no switches founds in %s", imp))
	}

	theSwitch, err := findSwitch(prog.Fset, astf, line, offset, switches)
	if err != nil {
		return fail(err)
	}

	st, err := typeOf(theSwitch, pkg.Info)
	if err != nil {
		return fail(err)
	}

	nt, err := typeNameOf(st)
	if err != nil {
		return fail(err)
	}

	dpkg, err = definingPackage(nt, prog)
	if err != nil {
		return fail(err)
	}

	ct, err = getClosed(nt, prog.Fset, dpkg)
	if err != nil {
		return fail(err)
	}

	return prog.Fset, astf, theSwitch, pkg, dpkg, prog, ct, nil
}

func computeCasesToAdd(sw ast.Stmt, ct closed.Type, pkg, dpkg *loader.PackageInfo, imps importMap) (cases []ast.Expr, defaultCase *ast.CaseClause, err error) {
	fail := func(err error) ([]ast.Expr, *ast.CaseClause, error) {
		return nil, nil, err
	}

	block, isTypeSwitch := body(sw)
	used, noDefault := usedBy(block, pkg.Info.Types)
	if noDefault {
		defaultCase = mkDefault()
	}

	diffPkgs := pkg.Pkg != dpkg.Pkg

	if isTypeSwitch {
		var unused []types.Type
		addNil := false

		switch ct := ct.(type) {
		case *closed.Interface:
			unused, addNil = missingInterfaceCases(ct, used, diffPkgs)

		case *closed.EmptySum:
			unused, addNil = missingEmptyCases(ct, used, pkg.Pkg)

		default:
			return fail(fmt.Errorf("internal error: unexpected %T for type switch", ct))
		}

		if addNil {
			cases = append(cases, mkNil())
		}

		tp := newTypeSerializer(pkg.Pkg, imps)
		for _, u := range unused {
			x, err := tp.print(u)
			if err != nil {
				return fail(err)
			}
			cases = append(cases, x)
		}
	} else {
		enum, ok := ct.(*closed.Enum)
		if !ok {
			return fail(fmt.Errorf("internal error: unexpected %T for regular switch", ct))
		}

		unused, addZero, kind := missingEnumCases(enum, used, diffPkgs)

		if addZero {
			cases = append(cases, mkZero(kind))
		}

		pkgname := ""
		if diffPkgs {
			pkgname = imps.Name(dpkg.Pkg)
		}

		for _, u := range unused {
			//like with closed.Interface, prefer exported labels
			lbl := closedutil.FirstExportedLabel(u)
			if lbl == nil {
				lbl = u[0]
			}

			cases = append(cases, mkLabel(lbl.Name(), pkgname))
		}
	}

	return cases, defaultCase, nil
}
