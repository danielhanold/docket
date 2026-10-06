package harness

// DispatchPreambleForTest exposes the unexported routing rule to the external
// test package, so the trigger guard's non-vacuity check can name a line the
// rule population must contain without spelling it a second time.
const DispatchPreambleForTest = dispatchPreamble
