package generator

import (
	"fmt"
	"go/types"
	"log"
	"reflect"
	"strings"

	"golang.org/x/tools/go/packages"
)

func (f *Fake) loadPackages() error {
	log.Println("loading packages...")
	p, err := packages.Load(&packages.Config{
		Mode:  packages.LoadSyntax,
		Dir:   f.WorkingDirectory,
		Tests: true,
	}, f.TargetPackage)
	if err != nil {
		return err
	}
	for i := range p {
		if len(p[i].Errors) > 0 {
			if i == 0 {
				err = p[i].Errors[0]
			}
			for j := range p[i].Errors {
				log.Printf("error loading packages: %v", strings.TrimPrefix(fmt.Sprintf("%v", p[i].Errors[j]), "-: "))
			}
		}
	}
	if err != nil {
		return err
	}
	f.Packages = p
	log.Printf("loaded %v packages\n", len(f.Packages))
	return nil
}

func (f *Fake) findPackage() error {
	var target *types.TypeName
	var pkg *packages.Package
	for i := range f.Packages {
		if f.Packages[i].Types == nil || f.Packages[i].Types.Scope() == nil {
			continue
		}
		pkg = f.Packages[i]
		if f.Mode == Package {
			break
		}

		raw := pkg.Types.Scope().Lookup(f.TargetName)
		if raw != nil {
			if typeName, ok := raw.(*types.TypeName); ok {
				target = typeName
				break
			}
		}
		pkg = nil
	}
	if pkg == nil {
		switch f.Mode {
		case Package:
			return fmt.Errorf("cannot find package with name: %s", f.TargetPackage)
		case InterfaceOrFunction:
			return fmt.Errorf("cannot find package with target: %s", f.TargetName)
		}
	}
	f.Target = target
	f.Package = pkg
	f.TargetPackage = unvendor(pkg.PkgPath)
	t := f.AddImport(pkg.Name, f.TargetPackage)
	f.TargetAlias = t.Alias
	if f.Mode != Package {
		f.TargetName = target.Name()
	}

	if f.Mode == InterfaceOrFunction {
		if !f.IsInterface() && !f.IsFunction() {
			return fmt.Errorf("cannot generate an fake for %s because it is not an interface or function", f.TargetName)
		}
	}

	if f.IsInterface() {
		log.Printf("Found interface with name: [%s]\n", f.TargetName)
	}
	if f.IsFunction() {
		log.Printf("Found function with name: [%s]\n", f.TargetName)
	}
	if f.Mode == Package {
		log.Printf("Found package with name: [%s]\n", f.TargetPackage)
	}
	return nil
}

// addImportsFor inspects the given type and adds imports to the fake if importable
// types are found.
func (f *Fake) addImportsFor(typ types.Type) {
	if typ == nil {
		return
	}

	switch t := typ.(type) {
	case *types.Basic:
		return
	case *types.Pointer:
		f.addImportsFor(t.Elem())
	case *types.Map:
		f.addImportsFor(t.Key())
		f.addImportsFor(t.Elem())
	case *types.Chan:
		f.addImportsFor(t.Elem())
	case *types.Named:
		if t.Obj() != nil && t.Obj().Pkg() != nil {
			f.AddImport(t.Obj().Pkg().Name(), t.Obj().Pkg().Path())
		}
	case *types.Slice:
		f.addImportsFor(t.Elem())
	case *types.Array:
		f.addImportsFor(t.Elem())
	case *types.Interface:
		return
	case *types.Signature:
		f.addTypesForMethod(t)
	default:
		log.Printf("!!! WARNING: Missing case for type %s\n", reflect.TypeOf(typ).String())
	}
}

func typeFor(typ types.Type, importsMap map[string]Import) string {
	if typ == nil {
		return ""
	}
	return types.TypeString(typ, func(p *types.Package) string {
		imp, ok := importsMap[unvendor(p.Path())]
		if ok {
			return imp.Alias
		}
		return ""
	})
}
