//Command clvalid generates a method or func to validate that the value
//of a closed type is legal.
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"log"
	"os"
	"unicode"
	"unicode/utf8"

	"github.com/jimmyfrasche/closed/cmds/internal/tools"
)

func failOn(err error) {
	if err != nil {
		log.Fatalf("%s: %s", os.Args[0], err)
	}
}

func main() {
	log.SetFlags(0)

	tools.AddTagsFlagDefault()
	var (
		output = flag.String("o", "", "The `filename` to output")
		method = flag.String("name", "", "The name to use for the func/method")
		mkfunc = flag.Bool("func", false, "Create a function instead of a method")
	)
	flag.Usage = func() {
		log.Printf("usage: %s [flags] [importPath] Type\n", os.Args[0])
		flag.PrintDefaults()
		log.Println("\nusage notes:")
		log.Println("\t* importPath is only allowed if Type is not defined in current package.")
		log.Println("\t* if the type is from a different package or cannot have methods, -func is implicit.")
		log.Println("\t* -name defaults to 'legal' for methods and 'legal<Type>' for funcs.")
		log.Printf("\t* If -o is not provided it defaults to f_%s.go, where f is the name of the file containing the declaration for Type.\n", os.Args[0])
	}
	flag.Parse()
	args := flag.Args()
	switch len(args) {
	case 1, 2:
	default:
		flag.Usage()
		os.Exit(2)
	}

	//NB not done handling arguments, but require further information to continue.

	typeName, forPkg, fromPkg, err := packages(build.Default.BuildTags, args)
	failOn(err)

	T, err := LoadType(fromPkg.ImportPath, typeName)
	failOn(err)

	//writing a validator for a type in a different package,
	if forPkg != fromPkg {
		//cannot add method
		*mkfunc = true

		err := externalOkay(T)
		failOn(err)
	}

	imports, impnames, err := computeImports(forPkg.ImportPath, T.T)
	failOn(err)

	filesBuildTags, err := tools.BuildTagsFrom(T.DefinedInFile)
	failOn(err)

	if mustFunc(T.T) {
		*mkfunc = true
	}
	if *method == "" {
		*method = "legal"

		if *mkfunc {
			name := T.Name
			if !ast.IsExported(name) {
				r, sz := utf8.DecodeRuneInString(name)
				name = name[sz:]
				r = unicode.ToUpper(r)
				name = string(r) + name
			}
			*method = fmt.Sprintf("legal%s", name)
		}
	}
	if *output == "" {
		prefix := T.DefinedInFile[:len(*output)-3] //strip off ".go"
		*output = fmt.Sprintf("%s_%s", prefix, os.Args[0])
	}

	err = tools.OverwriteCheck(*output, os.Args[0])
	failOn(err)

	g := &Generator{
		ToolName: os.Args[0],

		T: T,

		FName: *method,
		Func:  *mkfunc,

		BuildTags: filesBuildTags,

		PackageName: forPkg.Name,

		ThisPackageImp: forPkg.ImportPath,
		ImportNames:    impnames,
		Imports:        imports,
	}

	err = tools.Gofmt(*output, g.Generate)
	failOn(err)
}
