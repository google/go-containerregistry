package main

import (
	"errors"
	"fmt"
	"go/format"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"runtime/pprof"

	"github.com/maxbrunsfeld/counterfeiter/v6/arguments"
	"github.com/maxbrunsfeld/counterfeiter/v6/command"
	"github.com/maxbrunsfeld/counterfeiter/v6/generator"
)

func main() {
	debug.SetGCPercent(-1)

	if err := run(); err != nil {
		fail("%v", err)
	}
}

func run() error {
	profile := os.Getenv("COUNTERFEITER_PROFILE") != ""
	if profile {
		p, err := filepath.Abs(filepath.Join(".", "counterfeiter.profile"))
		if err != nil {
			return err
		}
		f, err := os.Create(p)
		if err != nil {
			return err
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			return err
		}
		fmt.Printf("Profile: %s\n", p)
		defer pprof.StopCPUProfile()
	}

	log.SetFlags(log.Lshortfile)
	if !isDebug() {
		log.SetOutput(ioutil.Discard)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return errors.New("Error - couldn't determine current working directory")
	}

	var cache generator.Cacher
	if disableCache() {
		cache = &generator.FakeCache{}
	} else {
		cache = &generator.Cache{}
	}
	var invocations []command.Invocation
	var args *arguments.ParsedArguments
	args, _ = arguments.New(os.Args, cwd, filepath.EvalSymlinks, os.Stat)
	generateMode := false
	if args != nil {
		generateMode = args.GenerateMode
	}
	invocations, err = command.Detect(cwd, os.Args, generateMode)
	if err != nil {
		return err
	}

	for i := range invocations {
		a, err := arguments.New(invocations[i].Args, cwd, filepath.EvalSymlinks, os.Stat)
		if err != nil {
			return err
		}
		err = generate(cwd, a, cache)
		if err != nil {
			return err
		}
	}
	return nil
}

func isDebug() bool {
	return os.Getenv("COUNTERFEITER_DEBUG") != ""
}

func disableCache() bool {
	return os.Getenv("COUNTERFEITER_DISABLECACHE") != ""
}

func generate(workingDir string, args *arguments.ParsedArguments, cache generator.Cacher) error {
	if err := reportStarting(workingDir, args.OutputPath, args.FakeImplName); err != nil {
		return err
	}

	b, err := doGenerate(workingDir, args, cache)
	if err != nil {
		return err
	}

	if err := printCode(b, args.OutputPath, args.PrintToStdOut); err != nil {
		return err
	}
	fmt.Fprint(os.Stderr, "Done\n")
	return nil
}

func doGenerate(workingDir string, args *arguments.ParsedArguments, cache generator.Cacher) ([]byte, error) {
	mode := generator.InterfaceOrFunction
	if args.GenerateInterfaceAndShimFromPackageDirectory {
		mode = generator.Package
	}
	f, err := generator.NewFake(mode, args.InterfaceName, args.PackagePath, args.FakeImplName, args.DestinationPackageName, workingDir, cache)
	if err != nil {
		return nil, err
	}
	return f.Generate(true)
}

func printCode(code []byte, outputPath string, printToStdOut bool) error {
	formattedCode, err := format.Source(code)
	if err != nil {
		return err
	}

	if printToStdOut {
		fmt.Println(string(formattedCode))
		return nil
	}
	os.MkdirAll(filepath.Dir(outputPath), 0777)
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("Couldn't create fake file - %v", err)
	}

	_, err = file.Write(formattedCode)
	if err != nil {
		return fmt.Errorf("Couldn't write to fake file - %v", err)
	}
	return nil
}

func reportStarting(workingDir string, outputPath, fakeName string) error {
	rel, err := filepath.Rel(workingDir, outputPath)
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("Writing `%s` to `%s`... ", fakeName, rel)
	if isDebug() {
		msg = msg + "\n"
	}
	fmt.Fprint(os.Stderr, msg)
	return nil
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
