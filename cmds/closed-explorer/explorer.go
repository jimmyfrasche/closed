//Command closed-explorer analyzes a package and prints its closed types to stdout.
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"log"
	"os"
	"strings"

	"github.com/jimmyfrasche/closed"
)

func failOn(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	log.SetFlags(0)

	flag.Parse()

	if flag.NArg() == 0 {
		log.Fatal("no package specified")
	}

	bi, err := build.Import(flag.Arg(0), "", 0)
	failOn(err)

	fileCheck := func(fi os.FileInfo) bool {
		nm := fi.Name()
		for _, f := range bi.GoFiles {
			if f == nm {
				return true
			}
		}
		return false
	}

	fs := token.NewFileSet()

	pkgs, err := parser.ParseDir(fs, bi.Dir, fileCheck, 0)
	failOn(err)

	var files []*ast.File
	for _, f := range pkgs[bi.Name].Files {
		files = append(files, f)
	}

	cfg := types.Config{
		Importer: importer.Default(),
	}

	pkg, err := cfg.Check(bi.ImportPath, fs, files, nil)
	failOn(err)

	vs, err := closed.InPackage(fs, files, pkg)
	failOn(err)

	for _, v := range vs {
		switch v := v.(type) {
		case *closed.Enum:
			fmt.Println("Enum:", v.Types()[0].Name())
			for _, lbl := range v.Labels {
				var nms []string
				for _, c := range lbl {
					nms = append(nms, c.Name())
				}
				fmt.Printf("\t%s\n", strings.Join(nms, " = "))
			}
			fmt.Println()

		case *closed.Interface:
			fmt.Println("Closed iface:", v.Types()[0].Name())
			for _, m := range v.Members {
				fmt.Printf("\t%s\n", names(m))
			}
			fmt.Println()

		case *closed.InterfaceSum:
			fmt.Println("Sum iface:", v.Types()[0].Name())
			fmt.Println("\ttags methods:")
			for _, t := range v.TagMethods {
				fmt.Printf("\t\t%s\n", t)
			}
			if len(v.FalseMembers) > 0 {
				fmt.Println("\tfalse members:")
				for _, m := range v.FalseMembers {
					fmt.Printf("\t\t%s\n", names(m))
				}
			}
			fmt.Println("\tmembers:")
			for _, m := range v.Members {
				fmt.Printf("\t\t%s\n", names(m))
			}
			fmt.Println()

		case *closed.EmptySum:
			fmt.Println("Empty sum:", v.Types()[0].Name())
			if v.Nil {
				fmt.Println("\t<nil>")
			}
			for _, m := range v.Members {
				fmt.Printf("\t%s\n", types.TypeString(m, nil))
			}

		case *closed.OptionalStruct:
			fmt.Println("Optional struct:", v.Types()[0].Name())
			fmt.Printf("\tDiscriminant: %s\n", v.Discriminant.Name())
			fmt.Printf("\tOptional: %s\n", v.Field.Name())
			fmt.Println()

		default:
			log.Fatalf("need to update explorer, new type %T added", v)
		}
	}
}

func names(t *closed.TypeNamesAndType) string {
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
