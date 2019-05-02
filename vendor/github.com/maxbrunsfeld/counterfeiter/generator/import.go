package generator

import (
	"log"
	"sort"
	"strings"
)

// Import is a package import with the associated alias for that package.
type Import struct {
	Alias string
	Path  string
}

// AddImport creates an import with the given alias and path, and adds it to
// Fake.Imports.
func (f *Fake) AddImport(alias string, path string) Import {
	path = unvendor(strings.TrimSpace(path))
	alias = strings.TrimSpace(alias)
	for i := range f.Imports {
		if f.Imports[i].Path == path {
			return f.Imports[i]
		}
	}
	log.Printf("Adding import: %s > %s\n", alias, path)
	result := Import{
		Alias: alias,
		Path:  path,
	}
	f.Imports = append(f.Imports, result)
	return result
}

// SortImports sorts imports alphabetically.
func (f *Fake) sortImports() {
	sort.SliceStable(f.Imports, func(i, j int) bool {
		if f.Imports[i].Path == "sync" {
			return true
		}
		if f.Imports[j].Path == "sync" {
			return false
		}
		return f.Imports[i].Path < f.Imports[j].Path
	})
}

func unvendor(s string) string {
	// Devendorize for use in import statement.
	if i := strings.LastIndex(s, "/vendor/"); i >= 0 {
		return s[i+len("/vendor/"):]
	}
	if strings.HasPrefix(s, "vendor/") {
		return s[len("vendor/"):]
	}
	return s
}

func (f *Fake) hasDuplicateAliases() bool {
	hasDuplicates := false
	for _, imports := range f.aliasMap() {
		if len(imports) > 1 {
			hasDuplicates = true
			break
		}
	}
	return hasDuplicates
}

func (f *Fake) printAliases() {
	for i := range f.Imports {
		log.Printf("- %s > %s\n", f.Imports[i].Alias, f.Imports[i].Path)
	}
}

// disambiguateAliases ensures that all imports are aliased uniquely.
func (f *Fake) disambiguateAliases() {
	f.sortImports()
	if !f.hasDuplicateAliases() {
		return
	}

	log.Printf("!!! Duplicate import aliases found,...")
	log.Printf("aliases before disambiguation:\n")
	f.printAliases()
	var byAlias map[string][]Import
	for {
		byAlias = f.aliasMap()
		if !f.hasDuplicateAliases() {
			break
		}

		for i := range f.Imports {
			imports := byAlias[f.Imports[i].Alias]
			if len(imports) == 1 {
				continue
			}

			for j := 0; j < len(imports); j++ {
				if imports[j].Path == f.Imports[i].Path && j > 0 {
					f.Imports[i].Alias = f.Imports[i].Alias + string('a'+byte(j-1))
					if f.Imports[i].Path == f.TargetPackage {
						f.TargetAlias = f.Imports[i].Alias
					}
				}
			}
		}
	}

	log.Println("aliases after disambiguation:")
	f.printAliases()
}

func (f *Fake) aliasMap() map[string][]Import {
	result := map[string][]Import{}
	for i := range f.Imports {
		imports := result[f.Imports[i].Alias]
		result[f.Imports[i].Alias] = append(imports, f.Imports[i])
	}
	return result
}

func (f *Fake) importsMap() map[string]Import {
	f.disambiguateAliases()
	result := map[string]Import{}
	for i := range f.Imports {
		result[f.Imports[i].Path] = f.Imports[i]
	}
	return result
}
