package api

import (
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// structWithUnsupported has a field kind setScalar/schema cannot handle.
type weird struct {
	F   float64 `json:"f" in:"query" cli:"--f"`
	Un  string  // untagged json -> lowercased name
	Bad bool    `json:"bad" in:"query" cli:"--bad"`
}

func TestSetScalarErrors(t *testing.T) {
	op := &Op{Method: "GET", Path: "/x", Proto: &weird{}}
	// unsupported kind (float) via query
	r := httptest.NewRequest("GET", "/x?f=1.5", nil)
	in := op.NewInput()
	if err := BindHTTP(op.Fields(), in, r, nil); err == nil {
		t.Error("expected unsupported-kind error")
	}
	// bad boolean
	r = httptest.NewRequest("GET", "/x?bad=notabool", nil)
	in = op.NewInput()
	if err := BindHTTP(op.Fields(), in, r, nil); err == nil {
		t.Error("expected bool parse error")
	}
}

func TestJSONTypeAndName(t *testing.T) {
	if jsonType(reflect.Float64) != "string" { // default fallthrough
		t.Error("jsonType default")
	}
	if jsonType(reflect.String) != "string" || jsonType(reflect.Int) != "integer" || jsonType(reflect.Bool) != "boolean" {
		t.Error("jsonType basics")
	}
	// untagged field -> lowercased struct name
	fs := parseStruct(&weird{})
	found := false
	for _, f := range fs {
		if f.name == "un" {
			found = true
		}
	}
	if !found {
		t.Error("untagged field name not lowercased to 'un'")
	}
}

func TestAsStringAndStr(t *testing.T) {
	// EncodeRequest exercises asString on int/bool/string/slice/ptr.
	type e struct {
		S  string   `json:"s" in:"query" cli:"--s"`
		N  int      `json:"n" in:"query" cli:"--n"`
		B  bool     `json:"b" in:"query" cli:"--b"`
		L  []string `json:"l" in:"query" cli:"--l"`
		P  *string  `json:"p" in:"query" cli:"--p"`
		PN *int     `json:"pn" in:"body" cli:"--pn"`
	}
	op := &Op{Method: "GET", Path: "/x", Proto: &e{}}
	pv := "pval"
	pn := 5
	in := &e{S: "sv", N: 7, B: true, L: []string{"a", "b"}, P: &pv, PN: &pn}
	_, q, body := EncodeRequest(op.Fields(), in, op.Method, op.Path)
	if q.Get("s") != "sv" || q.Get("n") != "7" || q.Get("b") != "true" || q.Get("l") != "a,b" || q.Get("p") != "pval" {
		t.Fatalf("query encode wrong: %v", q)
	}
	if pp, ok := body["pn"].(*int); !ok || *pp != pn { // *int body field
		t.Fatalf("body pn = %v", body["pn"])
	}
	// nil ptr fields are skipped (isZero true)
	empty := &e{}
	_, q2, _ := EncodeRequest(op.Fields(), empty, op.Method, op.Path)
	if len(q2) != 0 {
		t.Fatalf("empty encode should be empty, got %v", q2)
	}
}

func TestOneOfEmpty(t *testing.T) {
	// empty value passes any enum (no constraint when unset)
	if err := oneOf("status", "", []string{"open"}); err != nil {
		t.Error("empty value should pass")
	}
}

func TestParseCLIRestEmpty(t *testing.T) {
	// rest positional with no trailing words is fine.
	op := &Op{Method: "POST", Path: "/x/{id}", Proto: &CommentInput{}}
	in, err := op.ParseCLI([]string{"the-id"})
	if err != nil {
		t.Fatal(err)
	}
	if in.(*CommentInput).ID != "the-id" || in.(*CommentInput).Text != "" {
		t.Fatalf("parsed: %+v", in)
	}
}

func TestStrHelper(t *testing.T) {
	// non-string kind -> ""
	if str(reflect.ValueOf(42)) != "" {
		t.Error("non-string should be empty")
	}
	// nil pointer -> ""
	var pnil *string
	if str(reflect.ValueOf(pnil)) != "" {
		t.Error("nil ptr should be empty")
	}
	// valid *string -> value
	v := "hi"
	if str(reflect.ValueOf(&v)) != "hi" {
		t.Error("ptr string deref")
	}
}

func TestUsage(t *testing.T) {
	op := Ops().Lookup("create")
	u := op.Usage()
	for _, want := range []string{"Usage: tasks create", "Aliases: new", "<title...>",
		"(required)", "-p, --priority <value>", "Priority 0-4"} {
		if !strings.Contains(u, want) {
			t.Errorf("usage missing %q in:\n%s", want, u)
		}
	}
	// boolean flag has no <value> placeholder.
	up := Ops().Lookup("update").Usage()
	if !strings.Contains(up, "--claim ") && !strings.Contains(up, "--claim\n") {
		t.Errorf("update usage missing bare --claim:\n%s", up)
	}
	if strings.Contains(up[strings.Index(up, "--claim"):strings.Index(up, "--claim")+20], "<value>") {
		t.Error("--claim should not take a value")
	}
}
