package api

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

// field is the parsed metadata for one input-struct field. It is the atom of
// the meta-programming layer: one tagged field drives the HTTP wire location,
// the CLI flag/positional, the JSON-schema property, and validation across all
// three surfaces.
type field struct {
	idx      int
	name     string   // json name — used in body, query, path placeholder, MCP arg, schema
	loc      string   // "body" | "query" | "path"
	flags    []string // CLI flag spellings, e.g. ["-n","--limit"]; empty when not a flag
	arg      bool     // CLI positional
	rest     bool     // CLI positional that captures all remaining words (joined by space)
	desc     string   // description (MCP schema + CLI help)
	required bool
	kind     reflect.Kind // element kind (pointer/slice dereferenced): String|Int|Bool
	isPtr    bool
	isSlice  bool
}

// parseStruct reflects a pointer-to-struct prototype into field metadata.
func parseStruct(proto any) []field {
	t := reflect.TypeOf(proto).Elem()
	var fields []field
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}
		name := jsonName(sf)
		if name == "-" {
			continue
		}
		f := field{idx: i, name: name, loc: sf.Tag.Get("in"), desc: sf.Tag.Get("desc")}
		if f.loc == "" {
			f.loc = "body"
		}
		f.required = sf.Tag.Get("req") == "true"
		ft := sf.Type
		if ft.Kind() == reflect.Ptr {
			f.isPtr = true
			ft = ft.Elem()
		}
		if ft.Kind() == reflect.Slice {
			f.isSlice = true
			ft = ft.Elem()
		}
		f.kind = ft.Kind()
		switch cli := sf.Tag.Get("cli"); {
		case cli == "":
			// not a CLI-bound field
		case cli == "arg":
			f.arg = true
		case cli == "arg...":
			f.arg = true
			f.rest = true
		default:
			f.flags = strings.Split(cli, ",")
		}
		fields = append(fields, f)
	}
	return fields
}

func jsonName(sf reflect.StructField) string {
	tag := sf.Tag.Get("json")
	if tag == "" {
		return strings.ToLower(sf.Name)
	}
	if c := strings.IndexByte(tag, ','); c >= 0 {
		tag = tag[:c]
	}
	return tag
}

// ---- HTTP server side: bind an incoming request into the input struct ----

// BindHTTP fills the input struct from a request: JSON body (for write methods),
// path values, and query parameters, according to each field's location.
func BindHTTP(fields []field, in any, r *http.Request, decodeBody func(any) error) error {
	rv := reflect.ValueOf(in).Elem()
	// Body first (only fields with loc==body are present in the JSON payload;
	// json.Unmarshal ignores the rest).
	if decodeBody != nil {
		hasBody := false
		for _, f := range fields {
			if f.loc == "body" {
				hasBody = true
				break
			}
		}
		if hasBody {
			if err := decodeBody(in); err != nil {
				return err
			}
		}
	}
	for _, f := range fields {
		switch f.loc {
		case "path":
			if v := r.PathValue(f.name); v != "" {
				if err := setScalar(rv.Field(f.idx), f, v); err != nil {
					return err
				}
			}
		case "query":
			q := r.URL.Query()
			if !q.Has(f.name) {
				continue
			}
			if f.isSlice {
				if err := setSlice(rv.Field(f.idx), f, splitCSV(q.Get(f.name))); err != nil {
					return err
				}
			} else if err := setScalar(rv.Field(f.idx), f, q.Get(f.name)); err != nil {
				return err
			}
		}
	}
	return nil
}

// ---- CLI side: encode a populated input struct into an HTTP request ----

// EncodeRequest turns a populated input struct into (path, query, body-map).
// Path fields are substituted into the path template; query fields become the
// query string; body fields become a JSON object.
func EncodeRequest(fields []field, in any, method, pathTmpl string) (path string, query url.Values, body map[string]any) {
	rv := reflect.ValueOf(in).Elem()
	path = pathTmpl
	query = url.Values{}
	body = map[string]any{}
	for _, f := range fields {
		fv := rv.Field(f.idx)
		if isZero(fv) {
			continue
		}
		switch f.loc {
		case "path":
			path = strings.ReplaceAll(path, "{"+f.name+"}", url.PathEscape(asString(fv)))
		case "query":
			if f.isSlice {
				query.Set(f.name, strings.Join(asStrings(fv), ","))
			} else {
				query.Set(f.name, asString(fv))
			}
		default: // body
			body[f.name] = fv.Interface()
		}
	}
	return path, query, body
}

// ---- scalar/slice setters (shared by HTTP query, path, and CLI flags) ----

func setScalar(fv reflect.Value, f field, raw string) error {
	target := fv
	if f.isPtr {
		p := reflect.New(fv.Type().Elem())
		fv.Set(p)
		target = p.Elem()
	}
	switch f.kind {
	case reflect.String:
		target.SetString(raw)
	case reflect.Int, reflect.Int64:
		n, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("invalid integer for %q: %q", f.name, raw)
		}
		target.SetInt(int64(n))
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("invalid boolean for %q: %q", f.name, raw)
		}
		target.SetBool(b)
	default:
		return fmt.Errorf("unsupported field kind for %q", f.name)
	}
	return nil
}

func setSlice(fv reflect.Value, f field, vals []string) error {
	sl := reflect.MakeSlice(fv.Type(), 0, len(vals))
	for _, v := range vals {
		sl = reflect.Append(sl, reflect.ValueOf(v))
	}
	fv.Set(sl)
	return nil
}

func isZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Ptr, reflect.Slice, reflect.Map:
		return v.IsNil()
	default:
		return v.IsZero()
	}
}

func asString(v reflect.Value) string {
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Int, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Bool:
		return strconv.FormatBool(v.Bool())
	}
	return fmt.Sprint(v.Interface())
}

func asStrings(v reflect.Value) []string {
	out := make([]string, v.Len())
	for i := 0; i < v.Len(); i++ {
		out[i] = asString(v.Index(i))
	}
	return out
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
