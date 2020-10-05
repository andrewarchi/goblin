package goblin

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"reflect"
	"strconv"
	"testing"
	"testing/quick"
)

// TODO: install github.com/stretchr/testify
// and make these unit tests use something other than just ==

func TestRoundTripFloat(t *testing.T) {
	f := func(flt float64) bool {
		flt = math.Abs(flt)
		needed := fmt.Sprintf("%f", flt)
		gotten := TestExpr(needed)
		result, _ := strconv.ParseFloat(gotten["value"].(string), 64)
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
		gotten := TestFile(fix.goPath)
		needed, _ := ioutil.ReadFile(fix.jsonPath)

		var gottenJ, neededJ interface{}

		err := json.Unmarshal(gotten, &gottenJ)
		if err != nil {
			panic("error reading " + fix.goPath + ": " + err.Error())
		}

		err = json.Unmarshal(needed, &neededJ)
		if err != nil {
			panic("error reading " + fix.jsonPath + ": " + err.Error())
		}

		t.Run(fix.name, func(tt *testing.T) {
			if !reflect.DeepEqual(gottenJ, neededJ) {
				t.Error("equality comparison failed!")
			}
		})
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
		gottenText, _ := ioutil.ReadFile(fix.goPath)
		gotten := TestExpr(string(gottenText))
		needed, _ := ioutil.ReadFile(fix.jsonPath)

		var neededJ interface{}

		err := json.Unmarshal(needed, &neededJ)
		if err != nil {
			panic("error reading " + fix.jsonPath + ": " + err.Error())
		}

		t.Run(fix.name, func(tt *testing.T) {
			if !reflect.DeepEqual(gotten, neededJ) {
				t.Error("equality comparison failed!")
			}
		})
	}
}

func TestIota(t *testing.T) {
	gotten := TestExpr("iota")
	val := gotten["value"].(map[string]interface{})
	if val["type"] != "IOTA" {
		t.Error("Didn't parse iota as a literal")
	}

}

func TestTrue(t *testing.T) {
	gotten := TestExpr("true")
	if gotten["type"] != "BOOL" || gotten["value"] != "true" {
		t.Error("Didn't parse 'true' as true")
	}
}

func TestFalse(t *testing.T) {
	gotten := TestExpr("false")
	if gotten["type"] != "BOOL" || gotten["value"] != "false" {
		t.Error("Didn't parse 'false' as false")
	}
}

func TestProvidedImag(t *testing.T) {
	gotten := TestExpr("1.414i")
	if gotten["type"] != "IMAG" || gotten["value"] != "1.414i" {
		t.Error("Imaginary numbers not parsing correctly")
	}
}

func TestProvidedFloat(t *testing.T) {
	gotten := TestExpr("3.14")
	if gotten["value"].(string) != "3.14" {
		t.Error("Floats not parsing correctly")
	}
}

func TestCall(t *testing.T) {
	gotten := TestExpr("foo(bar)")
	if gotten["type"].(string) != "call" {
		t.Error("Function calls not parsing correctly")
	}
}

func TestRoundTripUInt(t *testing.T) {
	f := func(int uint64) bool {
		needed := fmt.Sprintf("%d", int)
		gotten := TestExpr(needed)
		return needed == gotten["value"]
	}

	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}
