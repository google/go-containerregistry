package main

import (
	"flag"
	"fmt"
	"go/format"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/maxbrunsfeld/counterfeiter/arguments"
	"github.com/maxbrunsfeld/counterfeiter/generator"
)

func main() {
	log.SetFlags(log.Lshortfile)
	if !isDebug() {
		log.SetOutput(ioutil.Discard)
	}
	flag.Parse()
	args := flag.Args()

	if len(args) < 1 {
		fail("%s", usage)
		return
	}

	argumentParser := arguments.NewArgumentParser(
		fail,
		cwd,
		filepath.EvalSymlinks,
		os.Stat,
	)
	parsedArgs := argumentParser.ParseArguments(args...)
	generate(cwd(), parsedArgs)
}

func isDebug() bool {
	debug := os.Getenv("COUNTERFEITER_DEBUG")
	return debug != ""
}

func generate(workingDir string, args arguments.ParsedArguments) {
	reportStarting(args.PrintToStdOut, args.OutputPath, args.FakeImplName)
	mode := generator.InterfaceOrFunction
	if args.GenerateInterfaceAndShimFromPackageDirectory {
		mode = generator.Package
	}
	f, err := generator.NewFake(mode, args.InterfaceName, args.PackagePath, args.FakeImplName, args.DestinationPackageName, workingDir)
	if err != nil {
		fail("%v", err)
	}
	b, err := f.Generate(true)
	if err != nil {
		fail("%v", err)
	}

	printCode(string(b), args.OutputPath, args.PrintToStdOut)
	reportDoneSimple(args.PrintToStdOut)
}

func printCode(code, outputPath string, printToStdOut bool) {
	newCode, err := format.Source([]byte(code))
	if err != nil {
		fail("%v", err)
	}

	code = string(newCode)

	if printToStdOut {
		fmt.Println(code)
	} else {
		os.MkdirAll(filepath.Dir(outputPath), 0777)
		file, err := os.Create(outputPath)
		if err != nil {
			fail("Couldn't create fake file - %v", err)
		}

		_, err = file.WriteString(code)
		if err != nil {
			fail("Couldn't write to fake file - %v", err)
		}
	}
}

func reportStarting(printToStdOut bool, outputPath, fakeName string) {
	rel, err := filepath.Rel(cwd(), outputPath)
	if err != nil {
		fail("%v", err)
	}

	var writer io.Writer
	if printToStdOut {
		writer = os.Stderr
	} else {
		writer = os.Stdout
	}

	msg := fmt.Sprintf("Writing `%s` to `%s`... ", fakeName, rel)
	if isDebug() {
		msg = msg + "\n"
	}
	fmt.Fprint(writer, msg)
}

func reportDoneSimple(printToStdOut bool) {
	var writer io.Writer
	if printToStdOut {
		writer = os.Stderr
	} else {
		writer = os.Stdout
	}

	fmt.Fprint(writer, "Done\n")
}

func reportDone(printToStdOut bool, outputPath, fakeName string) {
	rel, err := filepath.Rel(cwd(), outputPath)
	if err != nil {
		fail("%v", err)
	}

	var writer io.Writer
	if printToStdOut {
		writer = os.Stderr
	} else {
		writer = os.Stdout
	}

	fmt.Fprint(writer, fmt.Sprintf("Wrote `%s` to `%s`\n", fakeName, rel))
}

func cwd() string {
	dir, err := os.Getwd()
	if err != nil {
		fail("Error - couldn't determine current working directory")
	}
	return dir
}

func fail(s string, args ...interface{}) {
	fmt.Printf("\n"+s+"\n", args...)
	os.Exit(1)
}

var usage = `
USAGE
	counterfeiter
		[-o <output-path>] [-p] [--fake-name <fake-name>]
		[<source-path>] <interface> [-]

ARGUMENTS
	source-path
		Path to the file or directory containing the interface to fake.
		In package mode (-p), source-path should instead specify the path
		of the input package; alternatively you can use the package name
		(e.g. "os") and the path will be inferred from your GOROOT.

	interface
		If source-path is specified: Name of the interface to fake.
		If no source-path is specified: Fully qualified interface path of the interface to fake.
    If -p is specified, this will be the name of the interface to generate.

	example:
		# writes "FakeStdInterface" to ./packagefakes/fake_std_interface.go
		counterfeiter package/subpackage.StdInterface

	'-' argument
		Write code to standard out instead of to a file

OPTIONS
	-o
		Path to the file or directory for the generated fakes.
		This also determines the package name that will be used.
		By default, the generated fakes will be generated in
		the package "xyzfakes" which is nested in package "xyz",
		where "xyz" is the name of referenced package.

	example:
		# writes "FakeMyInterface" to ./mySpecialFakesDir/specialFake.go
		counterfeiter -o ./mySpecialFakesDir/specialFake.go ./mypackage MyInterface

	-p
		Package mode:  When invoked in package mode, counterfeiter
		will generate an interface and shim implementation from a
		package in your GOPATH.  Counterfeiter finds the public methods
		in the package <source-path> and adds those method signatures
		to the generated interface <interface-name>.

	example:
		# generates os.go (interface) and osshim.go (shim) in ${PWD}/osshim
		counterfeiter -p os
		# now generate fake in ${PWD}/osshim/os_fake (fake_os.go)
		go generate osshim/...

	--fake-name
		Name of the fake struct to generate. By default, 'Fake' will
		be prepended to the name of the original interface. (ignored in
		-p mode)

	example:
		# writes "CoolThing" to ./mypackagefakes/cool_thing.go
		counterfeiter --fake-name CoolThing ./mypackage MyInterface
`
