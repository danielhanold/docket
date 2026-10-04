package cli

import (
	"fmt"
	"reflect"

	"github.com/spf13/cobra"
)

// The JSON-file declaration annotations. declareJSONFile writes them when a
// command is built; TestPublishedRequestIsTheDecodedJSONFile reads them to
// prove each operation's published request (internal/app operationBindings)
// is exactly the type its decoder reads (ADR-0109's request surface).
const (
	jsonFileAnnotationFlag = "docket.jsonfile.flag"
	jsonFileAnnotationType = "docket.jsonfile.type"
)

// jsonFileTypeName is the type identity the declaration records and the guard
// compares: the full package path and type name, pointers dereferenced.
func jsonFileTypeName(t reflect.Type) string {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.PkgPath() + "." + t.Name()
}

// declareJSONFile is the single place a command declares "I read one strict
// JSON document of type T from --flag". It records T and the flag as
// annotations on cmd at construction time and returns the only decoder the
// command's RunE may use. The declaration and the decode share the type
// parameter T, so they cannot diverge. Every strict JSON decode in this
// package goes through here; TestJSONFileDecodesGoThroughTheRegisteringHelper
// fails on any other reference to decodeRequest.
//
// A second declaration on the same command panics: one operation reads at
// most one JSON file, and a silent overwrite would hide the first type from
// the guard. The panic fires when the command tree is built, which every
// cli test does.
func declareJSONFile[T any](cmd *cobra.Command, flag string) func(c *cobra.Command) (T, error) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	if prior, dup := cmd.Annotations[jsonFileAnnotationType]; dup {
		panic(fmt.Sprintf("command %q already declares JSON file type %s", cmd.Name(), prior))
	}
	cmd.Annotations[jsonFileAnnotationFlag] = flag
	cmd.Annotations[jsonFileAnnotationType] = jsonFileTypeName(reflect.TypeFor[T]())
	return func(c *cobra.Command) (T, error) {
		var v T
		source, _ := c.Flags().GetString(flag)
		err := decodeRequest(c.InOrStdin(), "--"+flag, source, &v)
		return v, err
	}
}
