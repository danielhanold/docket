package githubcli

import (
	"reflect"
	"testing"
)

// TestSelectMergeMethodPriority covers every effective-set combination: the
// selection is the fixed preference order rebase → merge → squash, else
// unavailable. Mutation discipline: each row that removes the previously
// selected method and requires the next one IS the mutation check — a selection
// that ignores the removal reddens the row.
func TestSelectMergeMethodPriority(t *testing.T) {
	cases := []struct {
		name string
		eff  methodSet
		want MergeMethod
		ok   bool
	}{
		{"all enabled -> rebase", methodSet{rebase: true, merge: true, squash: true}, MethodRebase, true},
		{"rebase only", methodSet{rebase: true}, MethodRebase, true},
		{"rebase+squash -> rebase", methodSet{rebase: true, squash: true}, MethodRebase, true},
		{"merge+squash -> merge", methodSet{merge: true, squash: true}, MethodMerge, true},
		{"merge only", methodSet{merge: true}, MethodMerge, true},
		{"squash only -> squash", methodSet{squash: true}, MethodSquash, true},
		{"empty -> unavailable", methodSet{}, "", false},
	}
	for _, c := range cases {
		got, ok := selectMergeMethod(c.eff)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: got (%q,%v) want (%q,%v)", c.name, got, ok, c.want, c.ok)
		}
	}
}

// TestMethodSetIntersect: repository permissions and branch rules compose by
// intersection, per element.
func TestMethodSetIntersect(t *testing.T) {
	a := methodSet{rebase: true, merge: true}
	b := methodSet{merge: true, squash: true}
	if got := a.intersect(b); got != (methodSet{merge: true}) {
		t.Fatalf("intersect: got %+v", got)
	}
	if got := a.intersect(methodSet{}); got != (methodSet{}) {
		t.Fatalf("intersect with empty must be empty: got %+v", got)
	}
}

// TestMergeFlag: the closed vocabulary renders exactly one gh flag per method;
// anything outside the vocabulary renders NOTHING (the act path guards on it).
func TestMergeFlag(t *testing.T) {
	for m, want := range map[MergeMethod]string{
		MethodRebase: "--rebase", MethodMerge: "--merge", MethodSquash: "--squash", MergeMethod("bogus"): "",
	} {
		if got := m.mergeFlag(); got != want {
			t.Errorf("mergeFlag(%q) = %q, want %q", m, got, want)
		}
	}
}

// TestMethodSetList: the diagnostic rendering names permitted methods in
// preference order.
func TestMethodSetList(t *testing.T) {
	got := methodSet{rebase: true, squash: true}.list()
	if !reflect.DeepEqual(got, []MergeMethod{MethodRebase, MethodSquash}) {
		t.Fatalf("list: got %v", got)
	}
	if l := (methodSet{}).list(); len(l) != 0 {
		t.Fatalf("empty set must list nothing, got %v", l)
	}
}
