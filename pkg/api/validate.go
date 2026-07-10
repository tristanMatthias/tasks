package api

import (
	"fmt"
	"reflect"

	"github.com/tristanMatthias/tasks/pkg/model"
)

// Validate enforces required fields and enum constraints on a populated input
// struct. It is shared by every surface, so the same rules apply to HTTP, MCP,
// and CLI callers — the tight control beads lacked.
func Validate(fields []field, in any) error {
	rv := reflect.ValueOf(in).Elem()
	for _, f := range fields {
		fv := rv.Field(f.idx)
		if f.required && isZero(fv) {
			return fmt.Errorf("missing required field %q", f.name)
		}
		if isZero(fv) {
			continue
		}
		if err := validateEnum(f.name, fv); err != nil {
			return err
		}
	}
	return nil
}

func validateEnum(name string, fv reflect.Value) error {
	switch name {
	case "status":
		return oneOf(name, str(fv), model.Statuses)
	case "issue_type", "type":
		return oneOf(name, str(fv), model.Types)
	case "priority":
		if fv.Kind() == reflect.Ptr {
			fv = fv.Elem()
		}
		p := int(fv.Int())
		if p < 0 || p > 4 {
			return fmt.Errorf("priority must be 0-4, got %d", p)
		}
	}
	return nil
}

// oneOf validates that every comma-separated part of v is in allowed. Splitting
// lets filter fields accept CSV (e.g. status="open,closed" for list) while
// single-value write fields still validate their one value.
func oneOf(name, v string, allowed []string) error {
	if v == "" {
		return nil
	}
	for _, part := range splitCSV(v) {
		ok := false
		for _, a := range allowed {
			if part == a {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("%s must be one of %v, got %q", name, allowed, part)
		}
	}
	return nil
}

func str(fv reflect.Value) string {
	if fv.Kind() == reflect.Ptr {
		if fv.IsNil() {
			return ""
		}
		fv = fv.Elem()
	}
	if fv.Kind() == reflect.String {
		return fv.String()
	}
	return ""
}
