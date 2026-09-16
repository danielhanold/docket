package nativeacceptance

import "testing"

func TestValue(t *testing.T) {
	if got := Value(); got != 1 {
		t.Fatalf("Value() = %d, want 1", got)
	}
}

func TestDouble(t *testing.T) {
	if got := Double(); got != 2 {
		t.Fatalf("Double() = %d, want 2", got)
	}
}
