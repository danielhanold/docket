package cli

import (
	"reflect"
	"sort"
	"strings"
)

// requestJSONKeys returns the sorted top-level JSON keys a closed request
// struct accepts — the exact set DisallowUnknownFields enforces. Embedded
// structs contribute their promoted keys; `json:"-"` fields contribute none.
func requestJSONKeys(dst any) []string {
	t := reflect.TypeOf(dst)
	for t.Kind() == reflect.Pointer {
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
			seen[tag] = true
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
