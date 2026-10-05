package app

import (
	"reflect"
	"sort"
	"strings"
)

// The docket: struct tag is the co-located, machine-readable vocabulary the
// request/result schema surface reports about a field — facts that otherwise
// live only in hand-written validators. It is a comma-separated option list,
// mirroring the encoding/json tag grammar, and the full vocabulary is fixed
// here so the spellings are settled once:
//
//	required            the field must be present/non-zero; a shape validator
//	                    mints an error finding on its absence. On a field inside
//	                    an optional object or a list element, it applies whenever
//	                    that object or element is sent.
//	success-only        the field is populated only on a successful result.
//	refusal-only        the field is populated only on a refusal/failure result.
//	enum=<vocabulary>   the field's value is drawn from the named set of
//	                    allowed values (e.g. enum=priority).
//
// Options combine on one field: docket:"required,enum=priority". All four options
// are now consumed: `required` by the request-shape validators (via
// requiredJSONKeys), and success-only/refusal-only/enum= by reflectFields
// (schema.go), which reads them into each field's Presence and Enum on the
// emitted schema surface.

// docketTagName is the struct-tag key the docket vocabulary lives under.
const docketTagName = "docket"

// hasDocketOption reports whether the field's docket: struct tag carries the
// given bare option (e.g. "required", "success-only"). Options are
// comma-separated, matching the encoding/json tag grammar.
func hasDocketOption(tag reflect.StructTag, opt string) bool {
	for _, o := range strings.Split(tag.Get(docketTagName), ",") {
		if strings.TrimSpace(o) == opt {
			return true
		}
	}
	return false
}

// docketEnumRef returns the vocabulary name a field's docket:"enum=<name>"
// option references, or "" when the tag carries no enum option.
func docketEnumRef(tag reflect.StructTag) string {
	const prefix = "enum="
	for _, o := range strings.Split(tag.Get(docketTagName), ",") {
		o = strings.TrimSpace(o)
		if strings.HasPrefix(o, prefix) {
			return strings.TrimPrefix(o, prefix)
		}
	}
	return ""
}

// jsonFieldKey derives the JSON key of one non-embedded struct field, the
// single copy of the key rules reflectFields, walkJSONKeys (behind
// RequestJSONKeys and requiredJSONKeys), and the tests' field lookup share:
// a `json:"-"` field and an untagged unexported field contribute nothing (ok
// is false), and an untagged exported field falls back to its Go field name.
// Promoting an embedded struct's fields is the caller's walk.
func jsonFieldKey(f reflect.StructField) (key string, ok bool) {
	key = strings.Split(f.Tag.Get("json"), ",")[0]
	if key == "-" || (key == "" && !f.IsExported()) {
		return "", false
	}
	if key == "" {
		key = f.Name
	}
	return key, true
}

// RequestJSONKeys returns the sorted top-level JSON keys a closed request
// struct accepts, which is the exact set DisallowUnknownFields enforces. The
// CLI lists them in an unknown-key refusal. It is walkJSONKeys keeping every
// field.
func RequestJSONKeys(prototype any) []string {
	return walkJSONKeys(prototype, func(reflect.StructField) bool { return true })
}

// requiredJSONKeys returns the sorted top-level JSON keys of prototype whose
// field carries docket:"required": the RequestJSONKeys walk filtered to
// required-tagged fields. The request-shape validator tests and the schema
// surface consume it.
func requiredJSONKeys(prototype any) []string {
	return walkJSONKeys(prototype, func(f reflect.StructField) bool {
		return hasDocketOption(f.Tag, "required")
	})
}

// walkJSONKeys is the one walk of a struct's top-level JSON keys behind
// RequestJSONKeys and requiredJSONKeys. It dereferences pointers, lets an
// embedded struct promote its fields, takes every other field's key from
// jsonFieldKey, and keeps a key only when keep reports true for its field.
// The result is sorted and non-nil.
func walkJSONKeys(prototype any, keep func(reflect.StructField) bool) []string {
	t := reflect.TypeOf(prototype)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	seen := map[string]bool{}
	var walk func(reflect.Type)
	walk = func(t reflect.Type) {
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.Anonymous && f.Type.Kind() == reflect.Struct {
				walk(f.Type)
				continue
			}
			key, ok := jsonFieldKey(f)
			if !ok {
				continue
			}
			if keep(f) {
				seen[key] = true
			}
		}
	}
	walk(t)
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
