// This file holds the second list a claude hook-entries target owns in its
// settings file — exact permission rules in permissions.allow — under the same
// byte-splice discipline as the hook entries: encoding/json only reads, and
// every byte docket does not own survives an insert or a removal.

package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// errAllowRulesInvalid reports a settings file whose shape leaves
// permissions.allow uneditable; callers take the skip path on it.
var errAllowRulesInvalid = errors.New("install: settings file is not a JSON object whose permissions value is an object and whose allow value is an array")

// readAllowRules owns the editability verdict for permissions.allow and returns
// its raw elements. ok=false means not editable, never "absent": an absent
// permissions or allow member, or a blank file, reads as editable and empty.
// It reads like encoding/json (last duplicate key wins), matching findMember.
func readAllowRules(src []byte) (elems []json.RawMessage, ok bool) {
	if len(bytes.TrimSpace(src)) == 0 {
		return nil, true
	}
	var top map[string]json.RawMessage
	if json.Unmarshal(src, &top) != nil || top == nil {
		return nil, false
	}
	raw, has := top["permissions"]
	if !has {
		return nil, true
	}
	var perms map[string]json.RawMessage
	if json.Unmarshal(raw, &perms) != nil || perms == nil {
		return nil, false
	}
	allow, has := perms["allow"]
	if !has {
		return nil, true
	}
	if !bytes.HasPrefix(bytes.TrimSpace(allow), []byte("[")) || json.Unmarshal(allow, &elems) != nil {
		return nil, false
	}
	return elems, true
}

// ruleIs reports whether one allow element decodes to exactly rule as a JSON
// string, whatever escapes spell it; non-string elements never match.
func ruleIs(raw json.RawMessage, rule string) bool {
	var s string
	return json.Unmarshal(raw, &s) == nil && s == rule
}

// allowRulePresent reports whether any element of permissions.allow is rule,
// so a hand-typed copy counts as present.
func allowRulePresent(elems []json.RawMessage, rule string) bool {
	for _, e := range elems {
		if ruleIs(e, rule) {
			return true
		}
	}
	return false
}

// insertAllowRules appends each missing rule, in order, as the last elements of
// permissions.allow, creating allow or permissions as the last member of its
// parent when absent and following the file's indent. A file already holding
// every rule comes back byte-identical, and a result that does not read back is
// refused.
func insertAllowRules(src []byte, rules []string) ([]byte, error) {
	elems, ok := readAllowRules(src)
	if !ok {
		return nil, errAllowRulesInvalid
	}
	var missing []string
	for _, r := range rules {
		if !allowRulePresent(elems, r) && !containsString(missing, r) {
			missing = append(missing, r)
		}
	}
	if len(missing) == 0 {
		return src, nil
	}
	if len(bytes.TrimSpace(src)) == 0 {
		out, err := json.MarshalIndent(map[string]map[string][]string{"permissions": {"allow": missing}}, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("install: encoding allow rules: %w", err)
		}
		return append(out, '\n'), nil
	}
	unit := indentUnit(src)
	out := src
	for _, r := range missing {
		next, err := insertNestedElement(out, "permissions", "allow", unit, r)
		if err != nil {
			return nil, err
		}
		out = next
	}
	if _, ok := readAllowRules(out); !ok {
		return nil, fmt.Errorf("install: adding allow rules produced an unreadable file")
	}
	return out, nil
}

// removeAllowRules cuts every element equal to one of rules, last rule first.
// An emptied allow goes with its member, and an emptied permissions then goes
// too, matching how hook entries leave their containers.
func removeAllowRules(src []byte, rules []string) ([]byte, error) {
	if _, ok := readAllowRules(src); !ok {
		return nil, errAllowRulesInvalid
	}
	out := src
	for i := len(rules) - 1; i >= 0; i-- {
		rule := rules[i]
		for {
			next, removed := removeNestedElement(out, "permissions", "allow", func(raw json.RawMessage) bool { return ruleIs(raw, rule) })
			if !removed {
				break
			}
			out = next
		}
	}
	if _, ok := readAllowRules(out); !ok {
		return nil, fmt.Errorf("install: removing allow rules produced an unreadable file")
	}
	return out, nil
}
