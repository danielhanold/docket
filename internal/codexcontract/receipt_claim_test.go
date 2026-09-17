package codexcontract

import (
	"encoding/json"
	"testing"
)

func TestFacadeReceiptRejectsIncompleteOrConflictingEnvelope(t *testing.T) {
	const valid = `{"protocol_version":1,"operation":"run.gate-claim","result":"applied","key":"k","decision":"gate-claimed","outcome":"WAITING","drive_id":"d","generation":"g","phase":"final","terminal":false}`
	for _, field := range []string{"protocol_version", "operation", "result", "key", "decision", "outcome", "drive_id", "generation", "phase", "terminal"} {
		t.Run(field, func(t *testing.T) {
			var doc map[string]any
			if err := json.Unmarshal([]byte(valid), &doc); err != nil {
				t.Fatal(err)
			}
			delete(doc, field)
			b, _ := json.Marshal(doc)
			if _, err := ParseReceipt("run.gate-claim", b, nil, 0, Assignment{}); err == nil {
				t.Fatalf("accepted missing %s", field)
			}
		})
	}
	for _, change := range []map[string]any{{"terminal": true}, {"outcome": "green"}, {"outcome": "HALTED"}, {"result": "invalid-state"}, {"decision": "gate-stop"}, {"drive": map[string]string{"generation": "wrong-envelope"}}, {"reason": "refused"}} {
		var doc map[string]any
		_ = json.Unmarshal([]byte(valid), &doc)
		for k, v := range change {
			doc[k] = v
		}
		b, _ := json.Marshal(doc)
		if _, err := ParseReceipt("run.gate-claim", b, nil, 0, Assignment{}); err == nil {
			t.Fatalf("accepted conflicting envelope: %v", change)
		}
	}
	if _, err := ParseReceipt("run.gate-claim", []byte(valid), nil, 1, Assignment{}); err == nil {
		t.Fatal("accepted nonzero successful claim")
	}
}

func TestFacadeReceiptPreservesTerminalClassification(t *testing.T) {
	for _, outcome := range []string{"WAITING", "PASSED", "FAILED"} {
		b, _ := json.Marshal(map[string]any{"protocol_version": 1, "operation": "run.gate-claim", "result": "applied", "key": "k", "decision": "gate-claimed", "outcome": outcome, "drive_id": "d", "generation": "g", "phase": "final", "terminal": false})
		r, err := ParseReceipt("run.gate-claim", b, nil, 0, Assignment{})
		if err != nil {
			t.Fatalf("%s: %v", outcome, err)
		}
		body, _ := json.Marshal(r)
		var got map[string]any
		_ = json.Unmarshal(body, &got)
		if got["classification"] != outcome || got["generation"] != "g" || got["authority"] != "owner" || got["next_operation"] != "gate.drive.advance" {
			t.Fatalf("lost claim semantics: %s", body)
		}
	}
}

func TestReceiptExpectationRejectsWrongIdentity(t *testing.T) {
	const body = `{"protocol_version":1,"operation":"run.gate-claim","result":"applied","key":"k","decision":"gate-claimed","outcome":"PASSED","drive_id":"d","generation":"g","phase":"final","terminal":false}`
	for _, e := range []ReceiptExpectation{{DriveID: "other", Phase: "final"}, {DriveID: "d", Phase: "build"}} {
		r, err := ParseReceipt("run.gate-claim", []byte(body), nil, 0, Assignment{}, e)
		if err == nil || r.Authority != "" || r.NextOperation != "" {
			t.Fatalf("identity mismatch retained authority: %+v %v", r, err)
		}
	}
	if _, err := ParseReceipt("run.gate-claim", []byte(body), nil, 0, Assignment{}, ReceiptExpectation{DriveID: "d", Phase: "final"}); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", body + body, body[:len(body)-1], `{"protocol_version":1,"protocol_version":1}`, `gate-claimed k PASSED d`} {
		if _, err := ParseReceipt("run.gate-claim", []byte(bad), nil, 0, Assignment{}); err == nil {
			t.Fatalf("accepted malformed receipt %q", bad)
		}
	}
}

func TestReceiptTransferAuthorityAndOrdinaryTerminalGeneration(t *testing.T) {
	for _, tc := range []struct{ op, authority, next string }{
		{"gate.drive.handoff", "handoff", ""}, {"gate.drive.claim", "owner", "gate.drive.advance"}, {"gate.drive.takeover", "owner", "gate.drive.advance"},
	} {
		b, _ := json.Marshal(map[string]any{"protocol_version": 1, "operation": tc.op, "result": "applied", "drive": map[string]any{"protocol_version": 1, "drive_id": "d", "generation": "g", "deadline": "2026-09-14T12:00:00Z", "outcome": "PASSED"}})
		r, err := ParseReceipt(tc.op, b, nil, 0, Assignment{})
		if err != nil || r.Authority != tc.authority || r.NextOperation != tc.next || r.Drive.Generation != "g" {
			t.Fatalf("%s: %+v %v", tc.op, r, err)
		}
	}
}
