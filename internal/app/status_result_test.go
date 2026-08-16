package app

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStatusResultEmptyCollectionsMarshalAsArrays(t *testing.T) {
	r := NewStatusResult(ResultApplied, StatusResult{})
	buf, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(buf)
	for _, want := range []string{`"changes":[]`, `"ready":[]`, `"records":[]`, `"findings":[]`} {
		if !strings.Contains(s, want) {
			t.Errorf("marshalled document missing %s: %s", want, s)
		}
	}
	if strings.Contains(s, "null") {
		t.Errorf("null leaked into protocol document: %s", s)
	}
}

func TestStatusResultEnvelope(t *testing.T) {
	r := NewStatusResult(ResultApplied, StatusResult{})
	env := r.Env()
	if env.ProtocolVersion != ProtocolVersion || env.Operation != "status" || env.Result != ResultApplied {
		t.Errorf("unexpected envelope: %+v", env)
	}
}

func TestStatusResultFailureShapeCarriesNoPartialReport(t *testing.T) {
	r := NewStatusResult(ResultExternalFailed, StatusResult{Reason: "unreachable-ref", Message: "boom"})
	if len(r.Changes)+len(r.Ready)+len(r.Records) != 0 {
		t.Errorf("failure result carried report sections: %+v", r)
	}
}
