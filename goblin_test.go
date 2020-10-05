package goblin

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"testing/quick"
)

// TODO: install github.com/stretchr/testify
// and make these unit tests use something other than just ==

func TestRoundTripFloat(t *testing.T) {
	f := func(flt float64) bool {
		flt = math.Abs(flt)
		want := fmt.Sprintf("%f", flt)
		got := TestExpr(want)
		result, _ := strconv.ParseFloat(got["value"].(string), 64)
		return flt == result
	}

	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

type Fixture struct {
	name     string
	goPath   string
	jsonPath string
}

func TestPackageFixtures(t *testing.T) {
	fixtures := []Fixture{
		{
			"helloworld",
			"fixtures/packages/helloworld/helloworld.go",
			"fixtures/packages/helloworld/helloworld.json",
		},
		{
			"simple type alias",
			"fixtures/packages/simpletypealias/simpletypealias.go",
			"fixtures/packages/simpletypealias/simpletypealias.json",
		},
		{
			"untyped top-level variable",
			"fixtures/packages/untypedvar/untyped.go",
			"fixtures/packages/untypedvar/untyped.json",
		},
		{
			"qualified type in function argument",
			"fixtures/packages/qualifiedtype/qualified.go",
			"fixtures/packages/qualifiedtype/qualified.json",
		},
		{
			"infinite for-loop",
			"fixtures/packages/emptyfor/empty.go",
			"fixtures/packages/emptyfor/empty.json",
		},
		{
			"select statement",
			"fixtures/packages/select/select.go",
			"fixtures/packages/select/select.json",
		},
		{
			"method declaration",
			"fixtures/packages/methoddecl/method.go",
			"fixtures/packages/methoddecl/method.json",
		},
		{
			"map with interface type",
			"fixtures/packages/interface_type/interface.go",
			"fixtures/packages/interface_type/interface.json",
		},
		{
			"empty function",
			"fixtures/packages/emptyfunc/empty.go",
			"fixtures/packages/emptyfunc/empty.json",
		},
	}

	for _, fix := range fixtures {
		got := TestFile(fix.goPath)
		want, _ := ioutil.ReadFile(fix.jsonPath)

		var gotJ, wantJ interface{}

		err := json.Unmarshal(got, &gotJ)
		if err != nil {
			t.Fatalf("error reading %s: %v", fix.goPath, err)
		}

		err = json.Unmarshal(want, &wantJ)
		if err != nil {
			t.Fatalf("error reading %s: %v", fix.jsonPath, err)
		}

		if !reflect.DeepEqual(gotJ, wantJ) {
			t.Errorf("equality comparison failed: %s", fix.name)
			dumpFail(t, fix, gotJ)
		}
	}
}

func TestExpressionFixtures(t *testing.T) {
	fixtures := []Fixture{
		{
			"cast to array",
			"fixtures/expressions/slicecast/slice.go.txt",
			"fixtures/expressions/slicecast/slice.json",
		},
		{
			"cast to pointer",
			"fixtures/expressions/ptrcast/ptr.go.txt",
			"fixtures/expressions/ptrcast/ptr.json",
		},
		{
			"map literal",
			"fixtures/expressions/mapliteral/map.go.txt",
			"fixtures/expressions/mapliteral/map.json",
		},
		{
			"single qualifier",
			"fixtures/expressions/singlequalifier/single.go.txt",
			"fixtures/expressions/singlequalifier/single.json",
		},
		{
			"double qualifier",
			"fixtures/expressions/doublequalifier/double.go.txt",
			"fixtures/expressions/doublequalifier/double.json",
		},
		{
			"cast to chan",
			"fixtures/expressions/chancast/chan.go.txt",
			"fixtures/expressions/chancast/chan.json",
		},
		{
			"cast with parenthesized type",
			"fixtures/expressions/parenintype/paren.go.txt",
			"fixtures/expressions/parenintype/paren.json",
		},
		{
			"adding two identifiers",
			"fixtures/expressions/addition/addition.go.txt",
			"fixtures/expressions/addition/addition.json",
		},
	}

	for _, fix := range fixtures {
		gotBytes, _ := ioutil.ReadFile(fix.goPath)
		got := TestExpr(string(gotBytes))
		want, _ := ioutil.ReadFile(fix.jsonPath)

		var wantJ interface{}

		err := json.Unmarshal(want, &wantJ)
		if err != nil {
			t.Fatalf("error reading %s: %v", fix.jsonPath, err)
		}

		if !reflect.DeepEqual(got, wantJ) {
			t.Errorf("equality comparison failed: %s", fix.name)
			dumpFail(t, fix, got)
		}
	}
}

func TestIota(t *testing.T) {
	got := TestExpr("iota")
	val := got["value"].(map[string]interface{})
	if val["type"] != "IOTA" {
		t.Error("Didn't parse iota as a literal")
	}
}

func TestTrue(t *testing.T) {
	got := TestExpr("true")
	if got["type"] != "BOOL" || got["value"] != "true" {
		t.Error("Didn't parse 'true' as true")
	}
}

func TestFalse(t *testing.T) {
	got := TestExpr("false")
	if got["type"] != "BOOL" || got["value"] != "false" {
		t.Error("Didn't parse 'false' as false")
	}
}

func TestProvidedImag(t *testing.T) {
	got := TestExpr("1.414i")
	if got["type"] != "IMAG" || got["value"] != "1.414i" {
		t.Error("Imaginary numbers not parsing correctly")
	}
}

func TestProvidedFloat(t *testing.T) {
	got := TestExpr("3.14")
	if got["value"].(string) != "3.14" {
		t.Error("Floats not parsing correctly")
	}
}

func TestCall(t *testing.T) {
	got := TestExpr("foo(bar)")
	if got["type"].(string) != "call" {
		t.Error("Function calls not parsing correctly")
	}
}

func TestRoundTripUInt(t *testing.T) {
	f := func(ui uint64) bool {
		want := fmt.Sprintf("%d", ui)
		got := TestExpr(want)
		return want == got["value"]
	}

	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func dumpFail(t *testing.T, fix Fixture, got interface{}) {
	t.Helper()
	f, err := os.Create(strings.TrimSuffix(fix.jsonPath, ".json") + ".got.json")
	if err != nil {
		t.Error(err)
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(got); err != nil {
		t.Error(err)
	}
}
