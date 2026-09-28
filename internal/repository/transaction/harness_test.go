package transaction

// itoa is a tiny local integer formatter that keeps the corpus builders (now in
// harness_integration_test.go, behind the integration tag) and scope_test.go free
// of an fmt import for one field. It stays untagged because both builds use it.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}
