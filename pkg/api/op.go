// Package api is the single source of truth for the task command surface. Each
// Op is declared once (an input struct + a handler over core) and the HTTP,
// MCP, and CLI adapters are generated from the registry by reflection — so a
// field or command added here propagates to every surface automatically.
package api

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/tristanMatthias/tasks/pkg/core"
)

// Op is one operation exposed on all surfaces.
type Op struct {
	Name    string   // canonical name: HTTP handler, MCP tool, CLI subcommand
	Aliases []string // extra CLI subcommand names
	Summary string   // description (MCP + CLI help)
	Method  string   // HTTP method: GET | POST | PATCH
	Path    string   // HTTP path template, e.g. "/api/v1/tasks/{id}/close"
	List    bool     // true if the handler returns a slice (affects CLI printing)
	Proto   any      // pointer to a zero input struct, e.g. &CreateInput{}
	Handle  func(c *core.Core, in any) (any, error)

	fields []field // parsed once via Fields()
}

// Fields returns (memoized) the parsed field metadata for the op's input struct.
func (o *Op) Fields() []field {
	if o.fields == nil {
		o.fields = parseStruct(o.Proto)
	}
	return o.fields
}

// NewInput allocates a fresh pointer-to-input-struct for this op.
func (o *Op) NewInput() any {
	return reflect.New(reflect.TypeOf(o.Proto).Elem()).Interface()
}

// HasBody reports whether the HTTP request carries a JSON body.
func (o *Op) HasBody() bool { return o.Method == "POST" || o.Method == "PATCH" }

// Write reports whether the op mutates state (anything but GET).
func (o *Op) Write() bool { return o.Method != "GET" }

// Schema builds the JSON Schema for the input struct (used by the MCP adapter).
// It is built by hand from field metadata so field descriptions and required
// flags are always present.
func (o *Op) Schema() *jsonschema.Schema {
	s := &jsonschema.Schema{Type: "object", Properties: map[string]*jsonschema.Schema{}}
	for _, f := range o.Fields() {
		ps := &jsonschema.Schema{Description: f.desc}
		switch {
		case f.isSlice:
			ps.Type = "array"
			ps.Items = &jsonschema.Schema{Type: jsonType(f.kind)}
		default:
			ps.Type = jsonType(f.kind)
		}
		s.Properties[f.name] = ps
		if f.required {
			s.Required = append(s.Required, f.name)
		}
	}
	return s
}

func jsonType(k reflect.Kind) string {
	switch k {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int64:
		return "integer"
	case reflect.Bool:
		return "boolean"
	default:
		return "string"
	}
}

// Usage renders per-command help (usage line, summary, positional arguments and
// flags with their descriptions) from the op's tagged input struct — the same
// single definition that drives the HTTP, MCP and CLI surfaces.
func (o *Op) Usage() string {
	var b strings.Builder
	fields := o.Fields()

	var args []field
	hasFlags := false
	for _, f := range fields {
		if f.arg {
			args = append(args, f)
		}
		if len(f.flags) > 0 {
			hasFlags = true
		}
	}

	b.WriteString("Usage: tasks " + o.Name)
	if hasFlags {
		b.WriteString(" [flags]")
	}
	for _, f := range args {
		if f.rest {
			b.WriteString(" <" + f.name + "...>")
		} else {
			b.WriteString(" <" + f.name + ">")
		}
	}
	b.WriteString("\n")
	if len(o.Aliases) > 0 {
		b.WriteString("Aliases: " + strings.Join(o.Aliases, ", ") + "\n")
	}
	if o.Summary != "" {
		b.WriteString("\n" + o.Summary + "\n")
	}

	if len(args) > 0 {
		b.WriteString("\nArguments:\n")
		for _, f := range args {
			b.WriteString(fmt.Sprintf("  %-22s %s%s\n", "<"+f.name+">", f.desc, reqMark(f)))
		}
	}
	if hasFlags {
		b.WriteString("\nFlags:\n")
		for _, f := range fields {
			if len(f.flags) == 0 {
				continue
			}
			spelling := strings.Join(f.flags, ", ")
			if f.kind != reflect.Bool || f.isSlice {
				spelling += " <value>"
			}
			b.WriteString(fmt.Sprintf("  %-26s %s%s\n", spelling, f.desc, reqMark(f)))
		}
	}
	b.WriteString("\nGlobal: --json (raw JSON), --silent (create: id only)\n")
	return b.String()
}

func reqMark(f field) string {
	if f.required {
		return " (required)"
	}
	return ""
}

// Registry is the ordered set of all operations.
type Registry []*Op

// Lookup finds an op by name or alias.
func (r Registry) Lookup(name string) *Op {
	for _, o := range r {
		if o.Name == name {
			return o
		}
		for _, a := range o.Aliases {
			if a == name {
				return o
			}
		}
	}
	return nil
}

// ---- CLI flag parsing (shared by the tasks CLI) ----

// ParseCLI fills a fresh input struct for op from CLI args, returning the input
// pointer. It supports `--flag val`, `--flag=val`, `-f val`, boolean flags, and
// positional args (in field order; a rest-field consumes the remainder).
func (o *Op) ParseCLI(args []string) (any, error) {
	in := o.NewInput()
	rv := reflect.ValueOf(in).Elem()
	flagByName := map[string]*field{}
	var positional []*field
	fields := o.Fields()
	for i := range fields {
		f := &fields[i]
		for _, name := range f.flags {
			flagByName[name] = f
		}
		if f.arg {
			positional = append(positional, f)
		}
	}

	var posVals []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") && a != "-" {
			name, inline, hasInline := a, "", false
			if j := strings.IndexByte(a, '='); j >= 0 {
				name, inline, hasInline = a[:j], a[j+1:], true
			}
			f, ok := flagByName[name]
			if !ok {
				return nil, fmt.Errorf("unknown flag: %s", name)
			}
			if f.kind == reflect.Bool && !f.isSlice {
				rv.Field(f.idx).SetBool(true)
				continue
			}
			val := inline
			if !hasInline {
				if i+1 >= len(args) {
					return nil, fmt.Errorf("flag %s needs a value", name)
				}
				i++
				val = args[i]
			}
			if err := assign(rv.Field(f.idx), f, val); err != nil {
				return nil, err
			}
			continue
		}
		posVals = append(posVals, a)
	}

	// Assign positionals in order; a rest-field swallows the remainder.
	for pi, f := range positional {
		if f.rest {
			if pi < len(posVals) {
				if err := assign(rv.Field(f.idx), f, strings.Join(posVals[pi:], " ")); err != nil {
					return nil, err
				}
			}
			posVals = nil
			break
		}
		if pi < len(posVals) {
			if err := assign(rv.Field(f.idx), f, posVals[pi]); err != nil {
				return nil, err
			}
		}
	}
	return in, nil
}

func assign(fv reflect.Value, f *field, val string) error {
	if f.isSlice {
		return setSlice(fv, *f, splitCSV(val))
	}
	return setScalar(fv, *f, val)
}
