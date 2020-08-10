package goblin

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"reflect"
	"strings"
	// "strconv"
)

var ShouldPanic bool = false
var TOPLEVEL_POSITION token.Position = token.Position{Filename: "toplevel", Offset: -1, Line: -1, Column: -1}
var INVALID_POSITION token.Position = token.Position{Filename: "unspecified", Offset: -1, Line: -1, Column: -1}

var tinfo *types.Info = nil

func Perish(pos token.Position, typ string, reason string) {
	if ShouldPanic {
		panic(pos.String() + ": " + reason)
	} else {
		res, _ := json.Marshal(map[string]interface{}{
			"error": map[string]interface{}{
				"type":     typ,
				"info":     reason,
				"position": DumpPosition(pos),
			},
		})
		os.Stderr.Write(res)
	}
	os.Exit(1)
}

func DumpPosition(p token.Position) map[string]interface{} {
	return map[string]interface{}{
		// We need these float64 conversions or our test cases will fail.
		// Believe me, I'm as angry about this as you are.
		"filename": p.Filename,
		"line":     float64(p.Line),
		"offset":   float64(p.Offset),
		"column":   float64(p.Column),
	}
}

// NOTE: this is brittle to changes to the types.BasicKind type.
var BasicKindStrings = [...]string{"Invalid", "Bool", "Int", "Int8", "Int16",
	"Int32", "Int64", "UInt", "UInt8", "UInt16", "UInt32", "UInt64",
	"UIntptr", "Float32", "Float64", "Complex64", "Complex128", "String",
	"UnsafePointer", "UntypedBool", "UntypedInt", "UntypedRune",
	"UntypedFloat", "UntypedComplex", "UntypedString", "UntypedNil"}

func BasicKindToString(k types.BasicKind) string {
	return BasicKindStrings[k]
}

func DumpVar(v *types.Var) map[string]interface{} {
	if v == nil {
		return nil
	}
	return map[string]interface{}{
		"name": v.Id(),
	}
}

func ConvertChanDir(dir types.ChanDir) ast.ChanDir {
	switch dir {
	case types.SendRecv:
		return ast.SEND | ast.RECV
	case types.SendOnly:
		return ast.SEND
	case types.RecvOnly:
		return ast.RECV
	default:
		panic("ConvertChanDir: unknown ChanDir")
	}
}

func getGoType(e ast.Expr) map[string]interface{} {
	if tinfo != nil {
		return DumpGoType(tinfo.Types[e].Type)
	} else {
		return nil
	}
}

// Impose a depth limit for now until we can detect and handle
// recursive types properly.
const TYPE_DEPTH_LIMIT int = 100

func dumpGoTypeAux(tp types.Type, d int) map[string]interface{} {
	if tp == nil {
		return nil
	}

	if d > TYPE_DEPTH_LIMIT {
		panic("dumpGoTypeAux: depth limit exceeded")
	}

	switch t := tp.(type) {
	case *types.Array:
		return map[string]interface{}{
			"type": "Array",
			"elem": dumpGoTypeAux(t.Elem(), d+1),
			"len":  t.Len(),
		}
	case *types.Basic:
		return map[string]interface{}{
			"type": "Basic",
			"kind": BasicKindToString(t.Kind()),
		}
	case *types.Chan:
		return map[string]interface{}{
			"type":      "Chan",
			"direction": DumpChanDir(ConvertChanDir(t.Dir())),
			"elem":      dumpGoTypeAux(t.Elem(), d+1),
		}
	case *types.Interface:
		// TODO: may need to call Complete() on t before doing this.
		methods := make([]map[string]interface{}, t.NumMethods())
		for i := 0; i < t.NumMethods(); i++ {
			m := t.Method(i)
			methods[i] = map[string]interface{}{
				"name": m.Name(),
				"type": dumpGoTypeAux(m.Type(), d+1),
			}
		}
		return map[string]interface{}{
			"type":    "Interface",
			"methods": methods,
		}
	case *types.Map:
		return map[string]interface{}{
			"type": "Map",
			"key":  dumpGoTypeAux(t.Key(), d+1),
			"elem": dumpGoTypeAux(t.Elem(), d+1),
		}
	case *types.Named:
		return map[string]interface{}{
			"type":       "Named",
			"underlying": dumpGoTypeAux(t.Underlying(), d+1),
		}
	case *types.Pointer:
		return map[string]interface{}{
			"type": "Pointer",
			"elem": dumpGoTypeAux(t.Elem(), d+1),
		}
	case *types.Signature:
		return map[string]interface{}{
			"type":     "Signature",
			"params":   dumpGoTypeAux(t.Params(), d+1),
			"recv":     DumpVar(t.Recv()),
			"results":  dumpGoTypeAux(t.Results(), d+1),
			"variadic": t.Variadic(),
		}
	case *types.Slice:
		return map[string]interface{}{
			"type": "Slice",
			"elem": dumpGoTypeAux(t.Elem(), d+1),
		}
	case *types.Struct:
		fields := make([]map[string]interface{}, t.NumFields())
		for i := 0; i < t.NumFields(); i++ {
			f := t.Field(i)
			fields[i] = map[string]interface{}{
				"name": f.Name(),
				"type": dumpGoTypeAux(f.Type(), d+1),
			}
		}
		return map[string]interface{}{
			"type":   "Struct",
			"fields": fields,
			// omit field tags for now
		}
	case *types.Tuple:
		fields := make([]map[string]interface{}, t.Len())
		for i := 0; i < t.Len(); i++ {
			f := t.At(i)
			fields[i] = map[string]interface{}{
				"name": f.Name(),
				"type": dumpGoTypeAux(f.Type(), d+1),
			}
		}
		return map[string]interface{}{
			"type":   "Tuple",
			"fields": fields,
		}
	default:
		fmt.Println("dumpGoTypeAux: unknown go type: ", tp)
		panic("")
	}
}

func DumpGoType(tp types.Type) map[string]interface{} {
	return dumpGoTypeAux(tp, 0)
}

func IdentKind(ident *ast.Ident) string {
	if tinfo != nil {
		o := tinfo.Uses[ident]
		switch o.(type) {
		case *types.Builtin:
			return "Builtin"
		case *types.Const:
			return "Const"
		case *types.Func:
			return "Func"
		case *types.Label:
			return "Label"
		case *types.Nil:
			return "Nil"
		case *types.PkgName:
			return "PkgName"
		case *types.TypeName:
			return "TypeName"
		case *types.Var:
			return "Var"
		default: // o is nil
		}
	}
	return "NoKind"
}

func DumpIdent(i *ast.Ident, fset *token.FileSet) map[string]interface{} {
	if i == nil {
		return nil
	}

	identKind := IdentKind(i)

	// This stuff only applies when type information isn't
	// available. Otherwise literals are handled by AttemptConst.
	asLiteral := map[string]interface{}{
		"kind":     "literal",
		"type":     "BOOL",
		"position": DumpPosition(fset.Position(i.Pos())),
	}
	switch i.Name {
	case "true":
		asLiteral["value"] = "true"
		return asLiteral

	case "false":
		asLiteral["value"] = "false"
		return asLiteral

	case "iota":
		asLiteral["type"] = "IOTA"
		return asLiteral

	}

	return map[string]interface{}{
		"kind":       "ident",
		"ident-kind": identKind,
		"value":      i.Name,
		"position":   DumpPosition(fset.Position(i.Pos())),
	}
}

func DumpArray(a *ast.ArrayType, fset *token.FileSet) map[string]interface{} {
	return map[string]interface{}{
		"kind":     "array",
		"length":   DumpExpr(a.Len, fset),
		"element":  DumpExprAsType(a.Elt, fset),
		"position": DumpPosition(fset.Position(a.Pos())),
	}
}

func withType(o map[string]interface{}, tp map[string]interface{}) map[string]interface{} {
	if tp != nil {
		o["go-type"] = tp
	}
	return o
}

func AttemptExprAsType(e ast.Expr, fset *token.FileSet) map[string]interface{} {
	if e == nil {
		return nil
	}

	if n, ok := e.(*ast.ParenExpr); ok {
		return AttemptExprAsType(n.X, fset)
	}

	tp := getGoType(e)

	if n, ok := e.(*ast.Ident); ok {
		return withType(map[string]interface{}{
			"kind":     "type",
			"type":     "identifier",
			"value":    DumpIdent(n, fset),
			"position": DumpPosition(fset.Position(e.Pos())),
		}, tp)
	}

	if n, ok := e.(*ast.SelectorExpr); ok {
		lhs := DumpExpr(n.X, fset)

		is_type := false
		if tinfo != nil {
			is_type = IdentKind(n.Sel) == "TypeName"
		} else {
			is_type = lhs["type"] == "identifier" && lhs["qualifier"] == nil
		}

		if is_type {
			return withType(map[string]interface{}{
				"kind":      "type",
				"type":      "identifier",
				"qualifier": lhs["value"],
				"value":     DumpIdent(n.Sel, fset),
				"position":  DumpPosition(fset.Position(e.Pos())),
			}, tp)
		}
	}

	if n, ok := e.(*ast.ArrayType); ok {
		if n.Len == nil {
			return withType(map[string]interface{}{
				"kind":     "type",
				"type":     "slice",
				"element":  DumpExprAsType(n.Elt, fset),
				"position": DumpPosition(fset.Position(e.Pos())),
			}, tp)
		} else {
			return withType(map[string]interface{}{
				"kind":     "type",
				"type":     "array",
				"element":  DumpExprAsType(n.Elt, fset),
				"length":   DumpExpr(n.Len, fset),
				"position": DumpPosition(fset.Position(e.Pos())),
			}, tp)
		}
	}

	if n, ok := e.(*ast.StarExpr); ok {
		return withType(map[string]interface{}{
			"kind":      "type",
			"type":      "pointer",
			"contained": DumpExprAsType(n.X, fset),
			"position":  DumpPosition(fset.Position(e.Pos())),
		}, tp)
	}

	if n, ok := e.(*ast.InterfaceType); ok {
		return withType(map[string]interface{}{
			"kind":       "type",
			"type":       "interface",
			"incomplete": n.Incomplete,
			"methods":    DumpFields(n.Methods, fset),
			"position":   DumpPosition(fset.Position(e.Pos())),
		}, tp)
	}

	if n, ok := e.(*ast.MapType); ok {
		return withType(map[string]interface{}{
			"kind":     "type",
			"type":     "map",
			"key":      DumpExprAsType(n.Key, fset),
			"value":    DumpExprAsType(n.Value, fset),
			"position": DumpPosition(fset.Position(e.Pos())),
		}, tp)
	}

	if n, ok := e.(*ast.ChanType); ok {
		return withType(map[string]interface{}{
			"kind":      "type",
			"type":      "chan",
			"direction": DumpChanDir(n.Dir),
			"value":     DumpExprAsType(n.Value, fset),
			"position":  DumpPosition(fset.Position(e.Pos())),
		}, tp)
	}

	if n, ok := e.(*ast.StructType); ok {
		return withType(map[string]interface{}{
			"kind":     "type",
			"type":     "struct",
			"fields":   DumpFields(n.Fields, fset),
			"position": DumpPosition(fset.Position(e.Pos())),
		}, tp)
	}

	if n, ok := e.(*ast.FuncType); ok {
		variadic := ExtractVariadic(n.Params)
		return withType(map[string]interface{}{
			"kind":     "type",
			"type":     "function",
			"params":   DumpFields(n.Params, fset),
			"variadic": AttemptField(variadic, fset),
			"results":  DumpFields(n.Results, fset),
			"position": DumpPosition(fset.Position(e.Pos())),
		}, tp)
	}

	if n, ok := e.(*ast.Ellipsis); ok {
		return withType(map[string]interface{}{
			"kind":  "type",
			"type":  "ellipsis",
			"value": DumpExprAsType(n.Elt, fset),
		}, tp)
	}

	return nil
}

func DumpExprAsType(e ast.Expr, fset *token.FileSet) map[string]interface{} {
	result := AttemptExprAsType(e, fset)

	if result != nil {
		return result
	}

	if e == nil {
		panic("unexpected nil Expr")
	}

	// bail out

	gotten := reflect.TypeOf(e).String()
	pos := fset.PositionFor(e.Pos(), true)
	Perish(pos, "unrecognized_type", gotten)
	panic("unreachable")
}

func DumpChanDir(d ast.ChanDir) string {
	switch d {
	case ast.SEND:
		return "send"

	case ast.RECV:
		return "recv"

	case ast.SEND | ast.RECV:
		return "both"
	}

	Perish(INVALID_POSITION, "internal_error", string(d))
	panic("unreachable")
}

func isBasicFloat(ty types.Type) bool {
	switch t := ty.(type) {
	case *types.Basic:
		return t.Kind() == types.Float32 || t.Kind() == types.Float64
	default:
		return false
	}
}

// Dump constant values as BasicConstExprs. Only possible when type
// information is available.
func AttemptConst(e ast.Expr, fset *token.FileSet) map[string]interface{} {
	tp := getGoType(e)
	if tp == nil {
		return nil
	}
	value := tinfo.Types[e].Value
	if value == nil {
		return nil
	}

	// Float literals end up being represented by integer
	// constants when possible. Here we convert them back to
	// floats.
	if isBasicFloat(tinfo.Types[e].Type) {
		value = constant.ToFloat(value)
	}

	return withType(map[string]interface{}{
		"kind":     "constant",
		"value":    DumpConstant(value),
		"position": DumpPosition(fset.Position(e.Pos())),
	}, tp)
}

func DumpConstant(value constant.Value) map[string]interface{} {
	switch value.Kind() {
	case constant.Bool:
		return map[string]interface{}{
			"type": "BOOL",
			"value": value.ExactString(),
		}
	case constant.String:
		return map[string]interface{}{
			"type": "STRING",
			"value": value.ExactString(),
		}
	case constant.Int:
		return map[string]interface{}{
			"type": "INT",
			"value": value.ExactString(),
		}
	case constant.Float:
		return map[string]interface{}{
			"type": "FLOAT",
			"numerator": DumpConstant(constant.Num(value)),
			"denominator": DumpConstant(constant.Denom(value)),
		}
	case constant.Complex:
		return map[string]interface{}{
			"type": "COMPLEX",
			"numerator": DumpConstant(constant.Num(value)),
			"denominator": DumpConstant(constant.Denom(value)),
		}
	case constant.Unknown:
	default:
		return nil
	}
	return nil
}


func DumpExpr(e ast.Expr, fset *token.FileSet) map[string]interface{} {
	if e == nil {
		return nil
	}

	c := AttemptConst(e, fset)
	if c != nil {
		return c
	}

	tp := getGoType(e)

	if _, ok := e.(*ast.ArrayType); ok {
		return DumpExprAsType(e, fset)
	}

	if n, ok := e.(*ast.Ident); ok {
		val := DumpIdent(n, fset)

		if val["type"] == "BOOL" {
			return val
		}

		return withType(map[string]interface{}{
			"kind":     "expression",
			"type":     "identifier",
			"value":    val,
			"position": DumpPosition(fset.Position(e.Pos())),
		}, tp)
	}

	if n, ok := e.(*ast.Ellipsis); ok {
		return withType(map[string]interface{}{
			"kind":  "expression",
			"type":  "ellipsis",
			"value": DumpExpr(n.Elt, fset),
		}, tp)
	}

	// is this the right place??
	if n, ok := e.(*ast.FuncLit); ok {
		variadic := ExtractVariadic(n.Type.Params)
		return withType(map[string]interface{}{
			"kind":     "literal",
			"type":     "function",
			"params":   DumpFields(n.Type.Params, fset),
			"variadic": AttemptField(variadic, fset),
			"results":  DumpFields(n.Type.Results, fset),
			"body":     DumpBlock(n.Body, fset),
			"position": DumpPosition(fset.Position(e.Pos())),
		}, tp)
	}

	if n, ok := e.(*ast.BasicLit); ok {
		return DumpBasicLit(n, fset)
	}

	if n, ok := e.(*ast.CompositeLit); ok {
		// unlike most 'type' nodes, types in CompositeLits can be omitted,
		// e.g. when you have a composite inside another one, which gives the
		// inner composites an implicit type:
		// bool[][] { { false, true }, { true, false }}

		return withType(map[string]interface{}{
			"kind":     "literal",
			"type":     "composite",
			"declared": AttemptExprAsType(n.Type, fset),
			"values":   DumpExprs(n.Elts, fset),
			"position": DumpPosition(fset.Position(e.Pos())),
		}, tp)
	}

	if b, ok := e.(*ast.BinaryExpr); ok {
		return withType(map[string]interface{}{
			"kind":     "expression",
			"type":     "binary",
			"left":     DumpExpr(b.X, fset),
			"right":    DumpExpr(b.Y, fset),
			"operator": b.Op.String(),
			"position": DumpPosition(fset.Position(b.Pos())),
		}, tp)
	}

	if n, ok := e.(*ast.IndexExpr); ok {
		return withType(map[string]interface{}{
			"kind":     "expression",
			"type":     "index",
			"target":   DumpExpr(n.X, fset),
			"index":    DumpExpr(n.Index, fset),
			"position": DumpPosition(fset.Position(e.Pos())),
		}, tp)
	}

	if n, ok := e.(*ast.StarExpr); ok {
		return withType(map[string]interface{}{
			"kind":   "expression",
			"type":   "star",
			"target": DumpExpr(n.X, fset),
		}, tp)
	}

	if n, ok := e.(*ast.CallExpr); ok {
		return DumpCall(n, fset)
	}

	if n, ok := e.(*ast.ParenExpr); ok {
		return withType(map[string]interface{}{
			"kind":     "expression",
			"type":     "paren",
			"target":   DumpExpr(n.X, fset),
			"position": DumpPosition(fset.Position(e.Pos())),
		}, tp)
	}

	if n, ok := e.(*ast.SelectorExpr); ok {
		lhs := DumpExpr(n.X, fset)
		// If the left hand side is just an identifier without a further qualifier,
		// assume that this is a qualified expression rather than a method call.
		// this is not correct in all cases, but ensuring correctness is outside
		// of the scope of a lowly parser such as goblin.
		// NOTE: this heuristic is only used when no type information is available.
		if tinfo == nil && lhs["type"] == "identifier" && lhs["qualifier"] == nil {
			return map[string]interface{}{
				"kind":      "expression",
				"type":      "identifier",
				"qualifier": lhs["value"],
				"value":     DumpIdent(n.Sel, fset),
				"position":  DumpPosition(fset.Position(e.Pos())),
			}
		}

		// If the lhs denotes a package name, this is a qualified identifier.
		if lhs["type"] == "identifier" {
			if lhs["value"].(map[string]interface{})["ident-kind"] == "PkgName" {
				return withType(map[string]interface{}{
					"kind":      "expression",
					"type":      "identifier",
					"qualifier": lhs["value"],
					"value":     DumpIdent(n.Sel, fset),
					"position":  DumpPosition(fset.Position(e.Pos())),
				}, tp)
			}
		}

		// Otherwise it's a field/method selector.
		return withType(map[string]interface{}{
			"kind":     "expression",
			"type":     "selector",
			"target":   lhs,
			"field":    DumpIdent(n.Sel, fset),
			"position": DumpPosition(fset.Position(e.Pos())),
		}, tp)
	}

	if n, ok := e.(*ast.TypeAssertExpr); ok {
		return withType(map[string]interface{}{
			"kind":     "expression",
			"type":     "type-assert",
			"target":   DumpExpr(n.X, fset),
			"asserted": AttemptExprAsType(n.Type, fset),
			"position": DumpPosition(fset.Position(e.Pos())),
		}, tp)
	}

	if n, ok := e.(*ast.UnaryExpr); ok {
		return withType(map[string]interface{}{
			"kind":     "expression",
			"type":     "unary",
			"target":   DumpExpr(n.X, fset),
			"operator": n.Op.String(),
			"position": DumpPosition(fset.Position(n.Pos())),
		}, tp)
	}

	if n, ok := e.(*ast.SliceExpr); ok {
		return withType(map[string]interface{}{
			"kind":     "expression",
			"type":     "slice",
			"target":   DumpExpr(n.X, fset),
			"low":      DumpExpr(n.Low, fset),
			"high":     DumpExpr(n.High, fset),
			"max":      DumpExpr(n.Max, fset),
			"three":    n.Slice3,
			"position": DumpPosition(fset.Position(e.Pos())),
		}, tp)
	}

	if n, ok := e.(*ast.KeyValueExpr); ok {
		return withType(map[string]interface{}{
			"kind":  "expression",
			"type":  "key-value",
			"key":   DumpExpr(n.Key, fset),
			"value": DumpExpr(n.Value, fset),
		}, tp)
	}

	if n, ok := e.(*ast.BadExpr); ok {
		pos := fset.PositionFor(n.From, true)
		Perish(pos, "internal_error", "encountered BadExpr")
	}

	typ := reflect.TypeOf(e).String()
	Perish(fset.Position(e.Pos()), "unexpected_node", typ)
	panic("unreachable")
}

func DumpExprs(exprs []ast.Expr, fset *token.FileSet) []interface{} {
	values := make([]interface{}, len(exprs))
	for i, v := range exprs {
		values[i] = DumpExpr(v, fset)
	}

	return values
}

// Given a token associated with a basic literal, return the BasicKind
// of its corresponding basic type.
func TokenBasicKind(tok token.Token) types.BasicKind {
	// token.INT, token.FLOAT, token.IMAG, token.CHAR, or token.STRING
	switch tok {
	case token.INT:
		return types.UntypedInt
	case token.FLOAT:
		return types.UntypedFloat
	case token.IMAG:
		return types.UntypedComplex
	case token.CHAR:
		return types.UntypedRune
	case token.STRING:
		return types.UntypedString
	default:
		return types.Invalid
	}
}

func TokenGoType(tok token.Token) types.Type {
	return types.Typ[TokenBasicKind(tok)]
}

func DumpBasicLit(l *ast.BasicLit, fset *token.FileSet) map[string]interface{} {
	if l == nil {
		return nil
	}

	return withType(map[string]interface{}{
		"kind":     "literal",
		"type":     l.Kind.String(),
		"value":    l.Value,
		"position": DumpPosition(fset.Position(l.Pos())),
	}, DumpGoType(TokenGoType(l.Kind)))
}

func AttemptField(f *ast.Field, fset *token.FileSet) map[string]interface{} {
	if f == nil {
		return nil
	} else {
		return DumpField(f, fset)
	}
}

func DumpField(f *ast.Field, fset *token.FileSet) map[string]interface{} {
	nameCount := 0
	if f.Names != nil {
		nameCount = len(f.Names)
	}

	names := make([]interface{}, nameCount)
	if f.Names != nil {
		for i, v := range f.Names {
			names[i] = DumpIdent(v, fset)
		}
	}

	return map[string]interface{}{
		"kind":          "field",
		"names":         names,
		"declared-type": DumpExprAsType(f.Type, fset),
		"tag":           DumpBasicLit(f.Tag, fset),
	}
}

func DumpFields(fs *ast.FieldList, fset *token.FileSet) []map[string]interface{} {
	if fs == nil {
		return nil
	}

	results := make([]map[string]interface{}, len(fs.List))
	for i, v := range fs.List {
		results[i] = DumpField(v, fset)
	}

	return results
}

func DumpCommentGroup(g *ast.CommentGroup, fset *token.FileSet) []string {
	if g == nil {
		return []string{}
	}

	result := make([]string, len(g.List))
	for i, v := range g.List {
		result[i] = v.Text
	}

	return result
}

func DumpTypeAlias(ts []*ast.TypeSpec, fset *token.FileSet) map[string]interface{} {
	binds := make([]interface{}, len(ts))
	for i, t := range ts {
		binds[i] = map[string]interface{}{
			"name":  DumpIdent(t.Name, fset),
			"value": DumpExprAsType(t.Type, fset),
		}
	}

	return map[string]interface{}{
		"kind":     "decl",
		"type":     "type-alias",
		"binds":    binds,
		"position": DumpPosition(fset.Position(ts[0].Pos())),
	}
}

func DumpCall(c *ast.CallExpr, fset *token.FileSet) map[string]interface{} {
	e := AttemptConst(c, fset)
	if e != nil {
		return e
	}
	tp := getGoType(c)

	if callee, ok := c.Fun.(*ast.Ident); ok {
		if callee.Name == "new" {
			return withType(map[string]interface{}{
				"kind":     "expression",
				"type":     "new",
				"argument": DumpExprAsType(c.Args[0], fset),
				"position": DumpPosition(fset.Position(c.Pos())),
			}, tp)
		}

		if callee.Name == "make" {
			return withType(map[string]interface{}{
				"kind":     "expression",
				"type":     "make",
				"argument": DumpExprAsType(c.Args[0], fset),
				"rest":     DumpExprs(c.Args[1:], fset),
				"position": DumpPosition(fset.Position(c.Pos())),
			}, tp)
		}
	}

	// try to parse the LHS as a type. if it succeeds and is *not* an identifier name,
	// it's a cast. currently, we don't have any heuristics for determining whether an
	// identifier is a typename (we don't even do the obvious cases like int8, float64
	// et cetera). such heuristics can't be perfectly accurate due to cross-module type
	// declarations, so it's probably more morally-correct, if less helpful, to treat them
	// as function calls and disambiguate them at a further stage.
	callee := AttemptExprAsType(c.Fun, fset)
	if callee != nil && callee["type"] != "identifier" {
		return withType(map[string]interface{}{
			"kind":       "expression",
			"type":       "cast",
			"target":     DumpExpr(c.Args[0], fset),
			"coerced-to": callee,
			"position":   DumpPosition(fset.Position(c.Pos())),
		}, tp)
	}

	// TODO: switch on the actual type here to disambiguate
	// between calls and casts? until then, the above is necessary
	// sometimes when the callee won't parse correctly as a normal
	// expression but only as a type.

	// callee := DumpExpr(c.Fun, fset)

	return withType(map[string]interface{}{
		"kind":      "expression",
		"type":      "call",
		"function":  DumpExpr(c.Fun, fset),
		"arguments": DumpExprs(c.Args, fset),
		"ellipsis":  c.Ellipsis != token.NoPos,
		"position":  DumpPosition(fset.Position(c.Pos())),
	}, tp)
}

func DumpImport(spec *ast.ImportSpec, fset *token.FileSet) map[string]interface{} {
	res := map[string]interface{}{
		"type":     "import",
		"doc":      DumpCommentGroup(spec.Doc, fset),
		"comments": DumpCommentGroup(spec.Comment, fset),
		"name":     DumpIdent(spec.Name, fset),
		"path":     strings.Trim(spec.Path.Value, "\""),
		"position": DumpPosition(fset.Position(spec.Pos())),
	}

	return res
}

func DumpValue(kind string, spec *ast.ValueSpec, fset *token.FileSet) map[string]interface{} {
	givenValues := []ast.Expr{}
	if spec.Values != nil {
		givenValues = spec.Values
	}

	processedValues := make([]interface{}, len(givenValues))
	for i, v := range givenValues {
		processedValues[i] = DumpExpr(v, fset)
	}

	processedNames := make([]interface{}, len(spec.Names))
	for i, v := range spec.Names {
		processedNames[i] = DumpIdent(v, fset)
	}

	return map[string]interface{}{
		"kind":          "spec",
		"type":          kind,
		"names":         processedNames,
		"declared-type": AttemptExprAsType(spec.Type, fset),
		"values":        processedValues,
		"comments":      DumpCommentGroup(spec.Comment, fset),
		"position":      DumpPosition(fset.Position(spec.Pos())),
	}
}

func TypeSpecsOfSpecs(specs []ast.Spec) []*ast.TypeSpec {
	ts := make([]*ast.TypeSpec, len(specs))
	for i, spec := range specs {
		ts[i] = spec.(*ast.TypeSpec)
	}
	return ts
}

func DumpGenDecl(decl *ast.GenDecl, fset *token.FileSet) map[string]interface{} {
	prettyToken := ""
	results := make([]interface{}, len(decl.Specs))
	switch decl.Tok {
	case token.TYPE:
		// EARLY RETURN
		return DumpTypeAlias(TypeSpecsOfSpecs(decl.Specs), fset)
	case token.IMPORT:
		prettyToken = "import"
		for i, v := range decl.Specs {
			results[i] = DumpImport(v.(*ast.ImportSpec), fset)
		}
	case token.CONST:
		prettyToken = "const"
		for i, v := range decl.Specs {
			results[i] = DumpValue("const", v.(*ast.ValueSpec), fset)
		}
	case token.VAR:
		prettyToken = "var"
		for i, v := range decl.Specs {
			results[i] = DumpValue("var", v.(*ast.ValueSpec), fset)
		}
	default:
		pos := fset.PositionFor(decl.Pos(), true)
		Perish(pos, "unrecognized_token", decl.Tok.String())
	}

	return map[string]interface{}{
		"kind":     "decl",
		"type":     prettyToken,
		"specs":    results,
		"position": DumpPosition(fset.Position(decl.Pos())),
	}
}

func DumpStmt(s ast.Stmt, fset *token.FileSet) interface{} {
	if s == nil {
		return nil
	}

	if n, ok := s.(*ast.ReturnStmt); ok {
		return map[string]interface{}{
			"kind":     "statement",
			"type":     "return",
			"values":   DumpExprs(n.Results, fset),
			"position": DumpPosition(fset.Position(n.Pos())),
		}
	}

	if n, ok := s.(*ast.AssignStmt); ok {
		if n.Tok == token.ASSIGN {
			return map[string]interface{}{
				"kind":     "statement",
				"type":     "assign",
				"left":     DumpExprs(n.Lhs, fset),
				"right":    DumpExprs(n.Rhs, fset),
				"position": DumpPosition(fset.Position(n.Pos())),
			}

		} else if n.Tok == token.DEFINE {
			return map[string]interface{}{
				"kind":     "statement",
				"type":     "define",
				"left":     DumpExprs(n.Lhs, fset),
				"right":    DumpExprs(n.Rhs, fset),
				"position": DumpPosition(fset.Position(n.Pos())),
			}
		} else {
			tok := n.Tok.String()
			return map[string]interface{}{
				"kind":     "statement",
				"type":     "assign-operator",
				"operator": tok[0 : len(tok)-1],
				"left":     DumpExprs(n.Lhs, fset),
				"right":    DumpExprs(n.Rhs, fset),
				"position": DumpPosition(fset.Position(n.Pos())),
			}
		}

	}

	if n, ok := s.(*ast.EmptyStmt); ok {
		return map[string]interface{}{
			"kind":     "statement",
			"type":     "empty",
			"position": DumpPosition(fset.Position(n.Pos())),
		}
	}

	if n, ok := s.(*ast.ExprStmt); ok {
		return map[string]interface{}{
			"kind":  "statement",
			"type":  "expression",
			"value": DumpExpr(n.X, fset),
		}
	}

	if n, ok := s.(*ast.LabeledStmt); ok {
		return map[string]interface{}{
			"kind":      "statement",
			"type":      "labeled",
			"label":     DumpIdent(n.Label, fset),
			"statement": DumpStmt(n.Stmt, fset),
			"position":  DumpPosition(fset.Position(n.Pos())),
		}
	}

	if n, ok := s.(*ast.BranchStmt); ok {
		result := map[string]interface{}{
			"kind":     "statement",
			"position": DumpPosition(fset.Position(n.Pos())),
		}

		switch n.Tok {
		case token.BREAK:
			result["type"] = "break"
			result["label"] = DumpIdent(n.Label, fset)

		case token.CONTINUE:
			result["type"] = "continue"
			result["label"] = DumpIdent(n.Label, fset)

		case token.GOTO:
			result["type"] = "goto"
			result["label"] = DumpIdent(n.Label, fset)

		case token.FALLTHROUGH:
			result["type"] = "fallthrough"

		}
		return result
	}

	if n, ok := s.(*ast.RangeStmt); ok {
		return map[string]interface{}{
			"kind":      "statement",
			"type":      "range",
			"key":       DumpExpr(n.Key, fset),
			"value":     DumpExpr(n.Value, fset),
			"target":    DumpExpr(n.X, fset),
			"is-assign": n.Tok == token.DEFINE,
			"body":      DumpBlock(n.Body, fset),
			"position":  DumpPosition(fset.Position(n.Pos())),
		}
	}
	if n, ok := s.(*ast.DeclStmt); ok {
		return map[string]interface{}{
			"kind":     "statement",
			"type":     "declaration",
			"target":   DumpDecl(n.Decl, fset),
			"position": DumpPosition(fset.Position(n.Pos())),
		}
	}

	if n, ok := s.(*ast.DeferStmt); ok {
		return map[string]interface{}{
			"kind":     "statement",
			"type":     "defer",
			"target":   DumpCall(n.Call, fset),
			"position": DumpPosition(fset.Position(n.Pos())),
		}
	}

	if n, ok := s.(*ast.IfStmt); ok {
		return map[string]interface{}{
			"kind":      "statement",
			"type":      "if",
			"init":      DumpStmt(n.Init, fset),
			"condition": DumpExpr(n.Cond, fset),
			"body":      DumpBlock(n.Body, fset),
			"else":      DumpStmt(n.Else, fset),
			"position":  DumpPosition(fset.Position(n.Pos())),
		}
	}

	if n, ok := s.(*ast.BlockStmt); ok {
		return DumpBlockAsStmt(n, fset)
	}

	if n, ok := s.(*ast.ForStmt); ok {
		return map[string]interface{}{
			"kind":      "statement",
			"type":      "for",
			"init":      DumpStmt(n.Init, fset),
			"condition": DumpExpr(n.Cond, fset),
			"post":      DumpStmt(n.Post, fset),
			"body":      DumpBlock(n.Body, fset),
			"position":  DumpPosition(fset.Position(n.Pos())),
		}
	}

	if n, ok := s.(*ast.GoStmt); ok {
		return map[string]interface{}{
			"kind":     "statement",
			"type":     "go",
			"target":   DumpCall(n.Call, fset),
			"position": DumpPosition(fset.Position(n.Pos())),
		}
	}

	if n, ok := s.(*ast.SendStmt); ok {
		return map[string]interface{}{
			"kind":     "statement",
			"type":     "send",
			"channel":  DumpExpr(n.Chan, fset),
			"value":    DumpExpr(n.Value, fset),
			"position": DumpPosition(fset.Position(n.Pos())),
		}
	}

	if n, ok := s.(*ast.SelectStmt); ok {
		return map[string]interface{}{
			"kind":     "statement",
			"type":     "select",
			"body":     DumpBlock(n.Body, fset),
			"position": DumpPosition(fset.Position(n.Pos())),
		}
	}

	if n, ok := s.(*ast.IncDecStmt); ok {
		return map[string]interface{}{
			"kind":      "statement",
			"type":      "crement",
			"target":    DumpExpr(n.X, fset),
			"operation": n.Tok.String(),
			"position":  DumpPosition(fset.Position(n.Pos())),
		}
	}

	if n, ok := s.(*ast.SwitchStmt); ok {
		return map[string]interface{}{
			"kind":      "statement",
			"type":      "switch",
			"init":      DumpStmt(n.Init, fset),
			"condition": DumpExpr(n.Tag, fset),
			"body":      DumpBlock(n.Body, fset),
			"position":  DumpPosition(fset.Position(n.Pos())),
		}
	}

	if n, ok := s.(*ast.TypeSwitchStmt); ok {
		return map[string]interface{}{
			"kind":     "statement",
			"type":     "type-switch",
			"init":     DumpStmt(n.Init, fset),
			"assign":   DumpStmt(n.Assign, fset),
			"body":     DumpBlock(n.Body, fset),
			"position": DumpPosition(fset.Position(n.Pos())),
		}
	}

	if n, ok := s.(*ast.CommClause); ok {
		stmts := make([]interface{}, len(n.Body))
		for i, v := range n.Body {
			stmts[i] = DumpStmt(v, fset)
		}

		return map[string]interface{}{
			"kind":      "statement",
			"type":      "select-clause",
			"statement": DumpStmt(n.Comm, fset),
			"body":      stmts,
			"position":  DumpPosition(fset.Position(n.Pos())),
		}

	}

	if n, ok := s.(*ast.CaseClause); ok {
		exprs := make([]interface{}, len(n.Body))
		for i, v := range n.Body {
			exprs[i] = DumpStmt(v, fset)
		}

		return map[string]interface{}{
			"kind":        "statement",
			"type":        "case-clause",
			"expressions": DumpExprs(n.List, fset),
			"body":        exprs,
			"position":    DumpPosition(fset.Position(n.Pos())),
		}
	}

	if n, ok := s.(*ast.BadStmt); ok {
		pos := fset.PositionFor(n.From, true)
		Perish(pos, "internal_error", "encountered BadStmt")
	}

	typ := reflect.TypeOf(s).String()
	pos := fset.PositionFor(s.Pos(), true)
	Perish(pos, "unexpected_node", typ)
	panic("unreachable")
}

func DumpBlock(b *ast.BlockStmt, fset *token.FileSet) []interface{} {
	if b == nil {
		return nil
	}
	results := make([]interface{}, len(b.List))
	for i, v := range b.List {
		results[i] = DumpStmt(v, fset)
	}

	return results
}

func DumpBlockAsStmt(b *ast.BlockStmt, fset *token.FileSet) map[string]interface{} {
	return map[string]interface{}{
		"kind":     "statement",
		"type":     "block",
		"body":     DumpBlock(b, fset),
		"position": DumpPosition(fset.Position(b.Pos())),
	}
}

// SIDE-EFFECT: if a variadic parameter is found, it is removed from
// the original FieldList.
func ExtractVariadic(params *ast.FieldList) *ast.Field {
	ps := params.List
	if len(ps) == 0 {
		return nil
	}
	p := ps[len(ps)-1]
	switch p.Type.(type) {
	case *ast.Ellipsis:
		params.List = params.List[:len(params.List)-1]
		return p
	default:
		return nil
	}
}

func DumpFuncDecl(f *ast.FuncDecl, fset *token.FileSet) map[string]interface{} {
	variadic := ExtractVariadic(f.Type.Params)
	return map[string]interface{}{
		"kind":     "decl",
		"type":     "function",
		"name":     DumpIdent(f.Name, fset),
		"body":     DumpBlock(f.Body, fset),
		"params":   DumpFields(f.Type.Params, fset),
		"variadic": AttemptField(variadic, fset),
		"results":  DumpFields(f.Type.Results, fset),
		"comments": DumpCommentGroup(f.Doc, fset),
		"position": DumpPosition(fset.Position(f.Pos())),
	}
}

func DumpMethodDecl(f *ast.FuncDecl, fset *token.FileSet) map[string]interface{} {
	variadic := ExtractVariadic(f.Type.Params)
	return map[string]interface{}{
		"kind":     "decl",
		"type":     "method",
		"receiver": DumpField(f.Recv.List[0], fset),
		"name":     DumpIdent(f.Name, fset),
		"body":     DumpBlock(f.Body, fset),
		"params":   DumpFields(f.Type.Params, fset),
		"variadic": AttemptField(variadic, fset),
		"results":  DumpFields(f.Type.Results, fset),
		"comments": DumpCommentGroup(f.Doc, fset),
		"position": DumpPosition(fset.Position(f.Pos())),
	}
}

func DumpDecl(n ast.Decl, fset *token.FileSet) map[string]interface{} {
	if decl, ok := n.(*ast.GenDecl); ok {
		return DumpGenDecl(decl, fset)
	}

	if decl, ok := n.(*ast.FuncDecl); ok {
		if decl.Recv == nil {
			return DumpFuncDecl(decl, fset)
		} else {
			return DumpMethodDecl(decl, fset)
		}
	}

	if decl, ok := n.(*ast.BadDecl); ok {
		pos := fset.PositionFor(decl.From, true)
		Perish(pos, "internal_error", "encountered BadDecl")
	}

	typ := reflect.TypeOf(n).String()
	pos := fset.PositionFor(n.Pos(), true)
	Perish(pos, "unexpected_node", typ)
	panic("unreachable")
}

func IsImport(d ast.Decl) bool {
	if decl, ok := d.(*ast.GenDecl); ok {
		return decl.Tok == token.IMPORT
	}
	return false
}

// AST nodes will be decorated with type information provided by the
// typeinfo argument if it's not nil.
func DumpFile(f *ast.File, path string, fset *token.FileSet, typeinfo *types.Info) map[string]interface{} {
	tinfo = typeinfo
	decls := []interface{}{}
	imps := []interface{}{}
	if f.Decls != nil {
		var ii int
		for ii = 0; ii < len(f.Decls); ii++ {
			if !IsImport(f.Decls[ii]) {
				break
			}
		}

		imports := f.Decls[0:ii]

		decls = make([]interface{}, len(f.Decls))
		for i, v := range f.Decls {
			decls[i] = DumpDecl(v, fset)
		}

		imps = make([]interface{}, len(imports))
		for i, v := range imports {
			imps[i] = DumpDecl(v, fset)
		}
	}

	allComments := make([][]string, len(f.Comments))
	for i, v := range f.Comments {
		allComments[i] = DumpCommentGroup(v, fset)
	}

	return map[string]interface{}{
		"kind":         "file",
		"path":         path,
		"package-name": DumpIdent(f.Name, fset),
		"comments":     DumpCommentGroup(f.Doc, fset),
		"all-comments": allComments,
		"declarations": decls,
		"imports":      imps,
	}
}

func DumpInitializer(init *types.Initializer, fset *token.FileSet) map[string]interface{} {
	vars := make([]map[string]interface{}, len(init.Lhs))
	for i, v := range init.Lhs {
		ident := ast.Ident{
			NamePos: v.Pos(),
			Name:    v.Name(),
			Obj:     nil,
		}
		vars[i] = map[string]interface{}{
			"kind":     "expression",
			"type":     "identifier",
			"value":    DumpIdent(&ident, fset),
			"position": DumpPosition(fset.Position(v.Pos())),
		}
	}

	return map[string]interface{}{
		"kind":  "statement",
		"type":  "initializer",
		"vars":  vars,
		"value": DumpExpr(init.Rhs, fset),
	}
}

// Initializers are dumped on a per-package basis.
func DumpInitializers(fset *token.FileSet, typeinfo *types.Info) []map[string]interface{} {
	tinfo = typeinfo
	initializers := make([]map[string]interface{}, len(tinfo.InitOrder))
	for i, init := range tinfo.InitOrder {
		initializers[i] = DumpInitializer(init, fset)
	}
	return initializers
}

func TestExpr(s string) map[string]interface{} {
	fset := token.NewFileSet() // positions are relative to fset

	f, err := parser.ParseExpr(s)
	if err != nil {
		panic(err.Error())
	}

	// Inspect the AST and print all identifiers and literals.
	return DumpExpr(f, fset)
}

func TestFile(p string) []byte {
	fset := token.NewFileSet()

	file, err := os.Open(p)
	if err != nil {
		return nil
	}
	info, err := file.Stat()
	if err != nil {
		return nil
	}

	size := info.Size()
	file.Close()

	fset.AddFile(p, -1, int(size))

	f, err := parser.ParseFile(fset, p, nil, 0)

	if err != nil {
		panic(err.Error())
	}

	// Inspect the AST and print all identifiers and literals.
	res, err := json.Marshal(DumpFile(f, p, fset, nil))

	if err != nil {
		panic(err.Error())
	}

	return res
}

func TestStmt(s string) []byte {
	fset := token.NewFileSet() // positions are relative to fset

	f, err := parser.ParseFile(fset, "stdin", "package p; func blah(foo int, bar float64) string { "+s+"}", 0)
	if err != nil {
		panic(err.Error())
	}

	// Inspect the AST and print all identifiers and literals.
	res, err := json.Marshal(DumpFile(f, s, fset, nil))

	if err != nil {
		panic(err.Error())
	}

	return res
}
