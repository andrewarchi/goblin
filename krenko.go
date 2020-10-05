package goblin

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/packages"
	"log"
	"os"
)

// This file contains the code for "Krenko, Mob Boss" (yes the MTG
// character). The code in goblin.go is used for dumping Go sources on
// a per-file basis. Krenko loads and typechecks a file along with all
// of its dependencies, dispatches goblin to generate serializable
// objects for every file, and constructs the final result.

// Currently the input is assumed to be a single file (which should
// contain a 'main' function). It should be straightforward to
// generalize to a multi-file package by pointing krenko at the
// package directory instead of a particular file.

func Load(file_path string) map[string]interface{} {
	fset := token.NewFileSet()

	// Parse the main file.
	f, err := parser.ParseFile(fset, file_path, nil, 0)
	if err != nil {
		log.Fatal(err) // parse error
	}

	// Use "ForCompiler" importer to load package sources instead
	// of precompiled files.
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}

	// Set up typechecker info for the main file. This is how we
	// specify which fields we want from the typechecker.
	info := types.Info{
		Types:     make(map[ast.Expr]types.TypeAndValue),
		Defs:      make(map[*ast.Ident]types.Object),
		Uses:      make(map[*ast.Ident]types.Object),
		InitOrder: []*types.Initializer{},
	}

	// Typecheck the main file.
	pkg, err := conf.Check("", fset, []*ast.File{f}, &info)
	if err != nil {
		log.Fatal(err) // type error
	}

	// Parse and typecheck all imported packages and their
	// dependencies.
	pkgs := import_packages(package_paths(pkg.Imports()))

	// Flatten the list of all packages in topological order so it
	// will be safe to process them in left-to-right order in
	// further analysis.
	pkgs_flat := []*packages.Package{}
	for _, p := range pkgs {
		pkgs_flat = accum_packages(pkgs_flat, p)
	}
	imports := DumpPackages(pkgs_flat)

	// Construct the final result object to be serialized.
	return map[string]interface{}{
		"name":    f.Name.Name,
		"package": DumpPackage(ConvertPackage(pkg, []string{f.Name.Name}, []*ast.File{f}, fset, &info)),
		"imports": imports,
	}
}

func package_paths(pkgs []*types.Package) []string {
	paths := make([]string, len(pkgs))
	for i, pkg := range pkgs {
		paths[i] = pkg.Path()
	}
	return paths
}

// Given a list of package names, use 'packages' to load and typecheck
// all of the packages along with the transitive closure of their
// dependencies.
func import_packages(pkg_names []string) []*packages.Package {
	if len(pkg_names) == 0 {
		return []*packages.Package{}
	}

	cfg := &packages.Config{Mode: packages.NeedName |
		packages.NeedSyntax | packages.NeedDeps |
		packages.NeedImports | packages.NeedTypes |
		packages.NeedTypesInfo | packages.NeedFiles}

	pkgs, err := packages.Load(cfg, pkg_names...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "import_packages: %v\n", err)
		os.Exit(1)
	}
	if packages.PrintErrors(pkgs) > 0 {
		fmt.Println("import_packages error")
		os.Exit(1)
	}

	return pkgs
}

// Gather transitive closure of import packages, avoiding duplicates.
func accum_packages(acc []*packages.Package, pkg *packages.Package) []*packages.Package {
	for _, p := range pkg.Imports {
		if !elem(p, acc) {
			acc = accum_packages(acc, p)
		}
	}
	acc = append(acc, pkg)
	return acc
}

func elem(x *packages.Package, l []*packages.Package) bool {
	for _, pkg := range l {
		if pkg.PkgPath == x.PkgPath {
			return true
		}
	}
	return false
}

// Use goblin's DumpFile.
func DumpPackage(pkg *packages.Package) map[string]interface{} {
	imports := []string{}
	for _, p := range pkg.Imports {
		imports = append(imports, p.PkgPath)
	}

	// Dump source files.
	files := make([]map[string]interface{}, len(pkg.Syntax))
	for i, f := range pkg.Syntax {
		files[i] = DumpFile(f, pkg.GoFiles[i], pkg.Fset, pkg.TypesInfo)
	}

	return map[string]interface{}{
		"name":         pkg.Name,
		"path":         pkg.PkgPath,
		"imports":      imports,
		"file-paths":   pkg.GoFiles,
		"files":        files,
		"initializers": DumpInitializers(pkg.Fset, pkg.TypesInfo),
	}
}

func DumpPackages(pkgs []*packages.Package) []map[string]interface{} {
	dumped := make([]map[string]interface{}, len(pkgs))
	for i, pkg := range pkgs {
		dumped[i] = DumpPackage(pkg)
	}
	return dumped
}

// Convert a types.Package (with some extra info) to a
// packages.Package.
func ConvertPackage(pkg *types.Package, file_names []string, files []*ast.File, fset *token.FileSet, info *types.Info) *packages.Package {
	return &packages.Package{
		Name:      pkg.Name(),
		PkgPath:   pkg.Path(),
		GoFiles:   file_names,
		Syntax:    files,
		Imports:   make(map[string]*packages.Package),
		Fset:      fset,
		TypesInfo: info,
	}
}
