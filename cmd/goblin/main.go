package main

import (
	"encoding/json"
	"flag"
	"github.com/GaloisInc/goblin"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
)

// Assuming you build with `make`, this variable will be filled in automatically
// (leaning on -ldflags -X).
var version string = "unspecified"

func main() {
	versionFlag := flag.Bool("v", false, "display goblin version")
	builtinDumpFlag := flag.Bool("builtin-dump", false, "use go/ast to dump the file, not JSON")
	panicFlag := flag.Bool("panic", false, "use panic() rather than JSON on error conditions")
	fileFlag := flag.String("file", "", "file to parse")
	stmtFlag := flag.String("stmt", "", "statement to parse")
	exprFlag := flag.String("expr", "", "expression to parse")
	fullFlag := flag.Bool("f", false, "parse and typecheck all imports (with file option)")

	flag.Parse()
	fset := token.NewFileSet() // positions are relative to fset

	if *panicFlag {
		goblin.ShouldPanic = true
	}

	if *versionFlag {
		println(version)
		return
	} else if *fileFlag != "" {
		// If full, use Load
		if *fullFlag {
			o := goblin.Load(*fileFlag)
			str, err := json.Marshal(o)
			if err != nil {
				log.Fatal(err)
			}
			os.Stdout.Write(str)
		} else {
			file, err := os.Open(*fileFlag)
			if err != nil {
				goblin.Perish(goblin.TOPLEVEL_POSITION, "path_error", err.Error())
			}
			info, err := file.Stat()
			if err != nil {
				goblin.Perish(goblin.TOPLEVEL_POSITION, "path_error", err.Error())
			}

			size := info.Size()
			file.Close()

			fset.AddFile(*fileFlag, -1, int(size))

			f, err := parser.ParseFile(fset, *fileFlag, nil, parser.ParseComments)
			if err != nil {
				goblin.Perish(goblin.INVALID_POSITION, "positionless_syntax_error", err.Error())
			}

			if *builtinDumpFlag {
				ast.Print(fset, f)
			} else {
				val, _ := json.Marshal(goblin.DumpFile(f, *fileFlag, fset, nil))
				os.Stdout.Write(val)
			}
		}
	} else if *exprFlag != "" {
		val, _ := json.Marshal(goblin.TestExpr(*exprFlag))
		os.Stdout.Write(val)
	} else if *stmtFlag != "" {
		val := goblin.TestStmt(*stmtFlag)
		os.Stdout.Write(val)
	} else {
		flag.PrintDefaults()
	}
}
