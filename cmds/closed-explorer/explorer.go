//Command closed-explorer analyzes a package and prints its closed types to stdout.
package main

import (
	"flag"
	"fmt"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"log"
	"os"
	"strings"

	"github.com/jimmyfrasche/closed"
	"github.com/jimmyfrasche/closed/cmds/internal/closedutil"
	"github.com/jimmyfrasche/closed/cmds/internal/tools"
)

func failOn(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	log.SetFlags(0)

	tools.AddTagsFlagDefault()
	flag.Parse()

	tags := build.Default.BuildTags
	imps, err := tools.GoList(tags, flag.Args())
	failOn(err)

	if len(imps) == 1 {
		err := explore(tags, imps[0], skipImport)
		failOn(err)
		return
	}

	failed := false
	for _, imp := range imps {
		err := explore(tags, imp, showImportsAndIndent)
		if err != nil {
			log.Print(err)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

type showImport bool

const (
	skipImport           showImport = false
	showImportsAndIndent showImport = true
)

func explore(tags []string, imp string, showImport showImport) error {
	ind := func() {
		if showImport {
			fmt.Print("\t")
		}
	}

	bi, err := build.Import(imp, "", 0)
	if err != nil {
		return err
	}

	fs := token.NewFileSet()

	pkgs, err := parser.ParseDir(fs, bi.Dir, tools.MakeFileCheck(bi.GoFiles), parser.ParseComments)
	if err != nil {
		return err
	}

	files := tools.FilesToSlice(pkgs[bi.Name])

	cfg := types.Config{
		Importer: importer.Default(),
	}

	pkg, err := cfg.Check(bi.ImportPath, fs, files, nil)
	if err != nil {
		return err
	}

	vs, err := closed.InPackage(fs, files, pkg)
	if err != nil {
		return err
	}

	if showImport {
		fmt.Printf("%s (%d)\n", bi.ImportPath, len(vs))
	}

	for _, v := range vs {
		switch v := v.(type) {
		case *closed.Enum:
			ind()
			fmt.Println("Enum:", name(v))
			if !v.NonZero && !closedutil.ContainsLabeledZero(v) {
				ind()
				fmt.Println("\t0")
			}
			for _, lbl := range v.Labels {
				ind()
				fmt.Printf("\t%s\n", labels(lbl))
			}
			fmt.Println()

		case *closed.Bitset:
			ind()
			fmt.Println("Bitset:", name(v))
			for _, f := range v.Flags {
				ind()
				fmt.Printf("\t%s\n", labels(f))
			}
			if len(v.OrFlags) > 1 {
				ind()
				fmt.Println("\t| flags")
				for _, f := range v.OrFlags {
					ind()
					fmt.Printf("\t\t%s\n", labels(f))
				}
			}
			fmt.Println()

		case *closed.Interface:
			ind()
			fmt.Println("Sum iface:", name(v))
			ind()
			fmt.Println("\ttags methods:")
			for _, t := range v.TagMethods {
				ind()
				fmt.Printf("\t\t%s\n", t)
			}
			if len(v.FalseMembers) > 0 {
				ind()
				fmt.Println("\tfalse members:")
				for _, m := range v.FalseMembers {
					ind()
					fmt.Printf("\t\t%s\n", typeNames(m))
				}
			}
			ind()
			fmt.Println("\tmembers:")
			if !v.NonNil {
				ind()
				fmt.Println("\t\t<nil>")
			}
			for _, m := range v.Members {
				ind()
				fmt.Printf("\t\t%s\n", typeNames(m))
			}
			fmt.Println()

		case *closed.EmptySum:
			ind()
			fmt.Println("Empty sum:", name(v))
			if v.Nil {
				ind()
				fmt.Println("\t<nil>")
			}
			for _, m := range v.Members {
				ind()
				fmt.Printf("\t%s\n", types.TypeString(m, nil))
			}

		case *closed.OptionalStruct:
			ind()
			fmt.Println("Optional struct:", name(v))
			ind()
			fmt.Printf("\tDiscriminant: %s\n", v.Discriminant.Name())
			ind()
			fmt.Printf("\tOptional: %s\n", v.Field.Name())
			fmt.Println()

		default:
			//this is serious so we just explode rather than spam stderr
			log.Fatalf("need to update explorer, new type %T added", v)
		}
	}

	return err
}

func name(t closed.Type) string {
	return t.Types()[0].Name()
}

func labels(lbl []*types.Const) string {
	var nms []string
	for _, c := range lbl {
		nms = append(nms, c.Name())
	}
	return strings.Join(nms, " = ")
}

func typeNames(t *closed.TypeNamesAndType) string {
	prefix := ""
	if _, ptr := t.Type.Underlying().(*types.Pointer); ptr {
		prefix = "*"
	}
	var acc []string
	for _, n := range t.TypeName {
		acc = append(acc, prefix+n.Name())
	}
	return strings.Join(acc, " = ")
}
