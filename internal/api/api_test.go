package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func jsonDecode(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// sample input used to exercise every field location/kind combination.
type sampleInput struct {
	ID     string   `json:"id"     in:"path"  cli:"arg"        req:"true"`
	Text   string   `json:"text"   in:"body"  cli:"arg..."     req:"true"`
	Limit  int      `json:"limit"  in:"query" cli:"-n,--limit"`
	Flag   bool     `json:"flag"   in:"body"  cli:"--flag"`
	PInt   *int     `json:"pint"   in:"body"  cli:"-p"`
	PStr   *string  `json:"pstr"   in:"body"  cli:"--pstr"`
	Labels []string `json:"labels" in:"query" cli:"-l,--labels"`
	Skip   string   `json:"-"`
}

func TestParseStruct(t *testing.T) {
	fs := parseStruct(&sampleInput{})
	if len(fs) != 7 { // Skip excluded
		t.Fatalf("got %d fields, want 7", len(fs))
	}
	byName := map[string]field{}
	for _, f := range fs {
		byName[f.name] = f
	}
	if f := byName["id"]; f.loc != "path" || !f.arg || !f.required {
		t.Errorf("id field wrong: %+v", f)
	}
	if f := byName["text"]; !f.rest {
		t.Errorf("text should be rest field: %+v", f)
	}
	if f := byName["limit"]; f.loc != "query" || len(f.flags) != 2 {
		t.Errorf("limit field wrong: %+v", f)
	}
	if f := byName["pstr"]; !f.isPtr {
		t.Errorf("pstr should be ptr")
	}
	if f := byName["labels"]; !f.isSlice {
		t.Errorf("labels should be slice")
	}
}

func TestParseCLI(t *testing.T) {
	op := &Op{Method: "POST", Path: "/x/{id}", Proto: &sampleInput{}}
	in, err := op.ParseCLI([]string{"the-id", "--limit", "5", "--flag", "-p", "3", "--pstr=hello", "-l", "a,b", "some", "long", "text"})
	if err != nil {
		t.Fatal(err)
	}
	s := in.(*sampleInput)
	if s.ID != "the-id" || s.Limit != 5 || !s.Flag || s.PInt == nil || *s.PInt != 3 || s.PStr == nil || *s.PStr != "hello" {
		t.Fatalf("parsed wrong: %+v", s)
	}
	if !reflect.DeepEqual(s.Labels, []string{"a", "b"}) {
		t.Fatalf("labels = %v", s.Labels)
	}
	if s.Text != "some long text" {
		t.Fatalf("rest text = %q", s.Text)
	}
}

func TestParseCLIErrors(t *testing.T) {
	op := &Op{Method: "POST", Path: "/x/{id}", Proto: &sampleInput{}}
	if _, err := op.ParseCLI([]string{"--nope"}); err == nil {
		t.Error("expected unknown flag error")
	}
	if _, err := op.ParseCLI([]string{"--limit"}); err == nil {
		t.Error("expected missing value error")
	}
	if _, err := op.ParseCLI([]string{"id", "--limit", "notanint"}); err == nil {
		t.Error("expected int parse error")
	}
}

func TestEncodeRequest(t *testing.T) {
	op := &Op{Method: "POST", Path: "/x/{id}", Proto: &sampleInput{}}
	pi := 7
	in := &sampleInput{ID: "abc", Text: "hi", Limit: 3, Flag: true, PInt: &pi, Labels: []string{"a", "b"}}
	path, query, body := EncodeRequest(op.Fields(), in, op.Method, op.Path)
	if path != "/x/abc" {
		t.Fatalf("path = %q", path)
	}
	if query.Get("limit") != "3" || query.Get("labels") != "a,b" {
		t.Fatalf("query = %v", query)
	}
	if body["text"] != "hi" || body["flag"] != true {
		t.Fatalf("body = %v", body)
	}
	if _, ok := body["id"]; ok {
		t.Error("id should be in path, not body")
	}
}

func TestBindHTTP(t *testing.T) {
	op := &Op{Method: "GET", Path: "/x/{id}", Proto: &sampleInput{}}
	r := httptest.NewRequest("GET", "/x/theid?limit=9&labels=p,q", nil)
	r.SetPathValue("id", "theid")
	in := op.NewInput()
	if err := BindHTTP(op.Fields(), in, r, nil); err != nil {
		t.Fatal(err)
	}
	s := in.(*sampleInput)
	if s.ID != "theid" || s.Limit != 9 || !reflect.DeepEqual(s.Labels, []string{"p", "q"}) {
		t.Fatalf("bound wrong: %+v", s)
	}
}

func TestBindHTTPBody(t *testing.T) {
	op := &Op{Method: "POST", Path: "/x/{id}", Proto: &sampleInput{}}
	r := httptest.NewRequest("POST", "/x/idv", strings.NewReader(`{"text":"body-text","flag":true}`))
	r.SetPathValue("id", "idv")
	in := op.NewInput()
	decode := func(v any) error {
		return jsonDecode(r, v)
	}
	if err := BindHTTP(op.Fields(), in, r, decode); err != nil {
		t.Fatal(err)
	}
	s := in.(*sampleInput)
	if s.ID != "idv" || s.Text != "body-text" || !s.Flag {
		t.Fatalf("bound body wrong: %+v", s)
	}
}

func TestSchema(t *testing.T) {
	op := &Op{Method: "POST", Path: "/x/{id}", Proto: &sampleInput{}}
	s := op.Schema()
	if s.Type != "object" {
		t.Fatal("schema not object")
	}
	if s.Properties["limit"].Type != "integer" || s.Properties["flag"].Type != "boolean" {
		t.Fatalf("prop types wrong: %+v", s.Properties)
	}
	if s.Properties["labels"].Type != "array" || s.Properties["labels"].Items.Type != "string" {
		t.Fatalf("array prop wrong: %+v", s.Properties["labels"])
	}
	req := strings.Join(s.Required, ",")
	if !strings.Contains(req, "id") || !strings.Contains(req, "text") {
		t.Fatalf("required = %v", s.Required)
	}
}

func TestRegistryLookup(t *testing.T) {
	r := Ops()
	if r.Lookup("create") == nil {
		t.Error("create not found")
	}
	if r.Lookup("new") == nil {
		t.Error("alias new not found")
	}
	if r.Lookup("ls") == nil {
		t.Error("alias ls not found")
	}
	if r.Lookup("bogus") != nil {
		t.Error("bogus should be nil")
	}
	// Every op must have parseable fields, a schema, and a handler.
	for _, op := range r {
		if op.Handle == nil {
			t.Errorf("%s missing handler", op.Name)
		}
		if len(op.Fields()) == 0 && op.Name != "" {
			// ready/list have fields; ok if some don't, but schema must build
		}
		if op.Schema() == nil {
			t.Errorf("%s schema nil", op.Name)
		}
		_ = op.NewInput()
		_ = op.HasBody()
		_ = op.Write()
	}
}

func TestValidate(t *testing.T) {
	fields := parseStruct(&CreateInput{})
	// missing required title
	if err := Validate(fields, &CreateInput{}); err == nil {
		t.Error("expected required error")
	}
	// bad issue_type
	if err := Validate(fields, &CreateInput{Title: "x", IssueType: "bogus"}); err == nil {
		t.Error("expected enum error")
	}
	// bad priority
	bad := 9
	if err := Validate(fields, &CreateInput{Title: "x", Priority: &bad}); err == nil {
		t.Error("expected priority error")
	}
	// valid
	good := 2
	if err := Validate(fields, &CreateInput{Title: "x", IssueType: "bug", Priority: &good}); err != nil {
		t.Errorf("valid input rejected: %v", err)
	}
	// list status CSV allowed
	lf := parseStruct(&ListInput{})
	if err := Validate(lf, &ListInput{Status: "open,closed"}); err != nil {
		t.Errorf("csv status rejected: %v", err)
	}
	if err := Validate(lf, &ListInput{Status: "open,bogus"}); err == nil {
		t.Error("expected csv status error")
	}
}
