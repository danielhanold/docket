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
//	                    mints an error finding on its absence. (Shipped: change
//	                    0399, Task 4.)
//	success-only        the field is populated only on a successful result.
//	                    (Vocabulary reserved; consumed by change 0399, Task 6.)
//	refusal-only        the field is populated only on a refusal/failure result.
//	                    (Vocabulary reserved; consumed by change 0399, Task 6.)
//	enum=<vocabulary>   the field's value is drawn from the named closed
//	                    vocabulary (e.g. enum=priority). (Vocabulary reserved;
//	                    consumed by change 0399, Task 6.)
//
// Options combine on one field: docket:"required,enum=priority". This task ships
// and consumes only `required`; the other options are declared now so their
// spellings do not churn when Task 6 ships them.

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

// requiredJSONKeys returns the sorted top-level JSON keys of prototype whose
// field carries docket:"required". It walks the same shape as the CLI's
// requestJSONKeys — embedded structs promote their fields; `json:"-"` and
// untagged unexported fields contribute nothing — filtered to required-tagged
// fields. It lives app-side because Task 5's schema generator consumes it.
func requiredJSONKeys(prototype any) []string {
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
			tag := strings.Split(f.Tag.Get("json"), ",")[0]
			if tag == "-" || tag == "" && !f.IsExported() {
				continue
			}
			if tag == "" {
				tag = f.Name
			}
			if hasDocketOption(f.Tag, "required") {
				seen[tag] = true
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
