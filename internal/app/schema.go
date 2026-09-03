package app

import (
	"fmt"
	"reflect"
	"strings"
)

// SchemaVersion identifies the schema-surface contract, versioned separately
// from protocol_version and capability_version; consumers refuse an
// unsupported value fail-closed, exactly as capability_version works.
const SchemaVersion = 1

// FieldDescriptor is one field of a request or result document. Key is the
// REAL JSON key; Type is the docket-native type word: "int", "string", "bool",
// "object" (Fields nested), "map[string]string". Repeated marks arrays (Type
// then describes the element). Enum names a document-level vocabulary. Presence
// is "" (always may appear), "success-only", or "refusal-only" (result fields
// only; from the docket tag). Required mirrors the docket:"required" tag.
type FieldDescriptor struct {
	Key      string            `json:"key"`
	Type     string            `json:"type"`
	Required bool              `json:"required,omitempty"`
	Repeated bool              `json:"repeated,omitempty"`
	Enum     string            `json:"enum,omitempty"`
	Presence string            `json:"presence,omitempty"`
	Fields   []FieldDescriptor `json:"fields,omitempty"`
}

// TypeDescriptor is one document side (request or result body).
type TypeDescriptor struct {
	Fields []FieldDescriptor `json:"fields"`
}

// reflectDescriptor walks a prototype struct into its descriptor. Embedded
// structs promote (the Envelope embed is handled by the registry, Task 7, which
// strips envelope keys from per-op results); pointer fields describe the
// element and are never required-by-shape; a nested struct (e.g.
// DesiredRelations under relations) recurses into Fields.
//
// Signature note (change 0399, Task 5): the plan draft signed this
// `reflectDescriptor(prototype any) TypeDescriptor`, but the kind-switch must
// fail closed on a shape it cannot describe (a map that is not
// map[string]string, an unhandled scalar kind). It therefore carries an error
// return — Task 7's registry accounting test relies on being able to surface an
// undescribable map here rather than a panic. The recursive worker is
// describeField; the struct-field walker is reflectFields.
func reflectDescriptor(prototype any) (TypeDescriptor, error) {
	t := reflect.TypeOf(prototype)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		return TypeDescriptor{}, fmt.Errorf("reflectDescriptor: prototype must be a struct, got %v", t)
	}
	fields, err := reflectFields(t)
	if err != nil {
		return TypeDescriptor{}, err
	}
	return TypeDescriptor{Fields: fields}, nil
}

// reflectFields walks a struct type's fields into descriptors, in declaration
// order. It mirrors requiredJSONKeys / the CLI's requestJSONKeys walk: an
// embedded struct promotes its fields inline; a `json:"-"` field and an
// untagged unexported field contribute nothing; an untagged exported field
// falls back to its Go field name. The docket tag (via Task 4's helpers)
// supplies Required, Presence, and Enum.
func reflectFields(t reflect.Type) ([]FieldDescriptor, error) {
	var out []FieldDescriptor
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous && f.Type.Kind() == reflect.Struct {
			promoted, err := reflectFields(f.Type)
			if err != nil {
				return nil, err
			}
			out = append(out, promoted...)
			continue
		}
		key := strings.Split(f.Tag.Get("json"), ",")[0]
		if key == "-" || (key == "" && !f.IsExported()) {
			continue
		}
		if key == "" {
			key = f.Name
		}
		fd, err := describeField(f.Type)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", key, err)
		}
		fd.Key = key
		if hasDocketOption(f.Tag, "required") {
			fd.Required = true
		}
		switch {
		case hasDocketOption(f.Tag, "success-only"):
			fd.Presence = "success-only"
		case hasDocketOption(f.Tag, "refusal-only"):
			fd.Presence = "refusal-only"
		}
		if enum := docketEnumRef(f.Tag); enum != "" {
			fd.Enum = enum
		}
		out = append(out, fd)
	}
	return out, nil
}

// describeField maps one Go type onto the shape half of a FieldDescriptor
// (Type, Repeated, and any nested Fields). It is the kind switch: an int kind is
// "int", a string kind "string", a bool "bool"; a slice or array sets Repeated
// and describes its element; a pointer describes its element (a pointer is never
// required-by-shape — required comes only from the docket tag); a struct is
// "object" with nested Fields; a map[string]string is "map[string]string". Any
// other map shape, or any other kind, fails closed with an error so the
// generator never silently mis-describes a field.
func describeField(t reflect.Type) (FieldDescriptor, error) {
	switch t.Kind() {
	case reflect.Pointer:
		return describeField(t.Elem())
	case reflect.Slice, reflect.Array:
		elem, err := describeField(t.Elem())
		if err != nil {
			return FieldDescriptor{}, err
		}
		elem.Repeated = true
		return elem, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return FieldDescriptor{Type: "int"}, nil
	case reflect.String:
		return FieldDescriptor{Type: "string"}, nil
	case reflect.Bool:
		return FieldDescriptor{Type: "bool"}, nil
	case reflect.Struct:
		fields, err := reflectFields(t)
		if err != nil {
			return FieldDescriptor{}, err
		}
		return FieldDescriptor{Type: "object", Fields: fields}, nil
	case reflect.Map:
		if t.Key().Kind() == reflect.String && t.Elem().Kind() == reflect.String {
			return FieldDescriptor{Type: "map[string]string"}, nil
		}
		return FieldDescriptor{}, fmt.Errorf("undescribable map shape %v", t)
	default:
		return FieldDescriptor{}, fmt.Errorf("undescribable kind %v for type %v", t.Kind(), t)
	}
}
