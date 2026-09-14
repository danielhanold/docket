// Execute this complete body in the controller's functions.exec after loading
// focused_scope and dispatch_seed. Pure data preparation: no tool/agent launch,
// filesystem write, transport adapter, or gate invocation occurs here.
function requireDispatch(condition, reason) {
  if (!condition) throw new Error('DISPATCH_INPUT_FAIL: ' + reason);
}
function nonemptyString(value) {
  return typeof value === 'string' && value.trim().length > 0;
}
function validateSources(scope, seed) {
  requireDispatch(scope && seed, 'scope or static seed missing');
  for (const key of ['scope_id', 'child_capability', 'parent_capability']) {
    requireDispatch(nonemptyString(scope[key]), 'missing scope field ' + key);
  }
  requireDispatch(scope.child_capability !== scope.parent_capability, 'capabilities must be distinct');
  requireDispatch(Object.keys(seed).sort().join(',') === 'assignment,entry_argv,entry_checker,task_input_file,task_input_sha256', 'seed field mismatch');
  for (const key of Object.keys(seed).filter(key => key !== 'entry_argv')) requireDispatch(nonemptyString(seed[key]), 'invalid seed field ' + key);
  requireDispatch(JSON.stringify(seed.entry_argv) === JSON.stringify(['python3', seed.entry_checker, '--inputs', seed.task_input_file, '--sha256', seed.task_input_sha256]), 'entry argv must be complete and explicitly invoke Python');
  requireDispatch(/^[a-f0-9]{64}$/.test(seed.task_input_sha256), 'invalid task-input digest');
  for (const key of ['task_input_file', 'entry_checker']) requireDispatch(seed[key].startsWith('/'), 'nonabsolute seed path');
}
function expectedWorkerMessage(scope, seed) {
  return JSON.stringify({
    task_input_file: seed.task_input_file,
    task_input_sha256: seed.task_input_sha256,
    entry_checker: seed.entry_checker,
    entry_argv: seed.entry_argv,
    scope_id: scope.scope_id,
    child_capability: scope.child_capability,
    gate_context: dispatchGateContext,
    run_epoch: dispatchRunEpoch,
    assignment: seed.assignment
  }, null, 2);
}
function validateDispatch(args, scope, seed) {
  validateSources(scope, seed);
  requireDispatch(args && Object.keys(args).sort().join(',') === 'agent_type,fork_turns,message,task_name', 'native argument fields mismatch');
  requireDispatch(args.agent_type === 'docket-build-standard' && args.task_name === 'focused_worker' && args.fork_turns === 'none', 'wrong native role or history mode');
  requireDispatch(typeof args.message === 'string', 'message must be text');
  let received;
  try { received = JSON.parse(args.message); } catch { throw new Error('DISPATCH_INPUT_FAIL: worker message is not structured JSON'); }
  requireDispatch(Object.keys(received).sort().join(',') === 'assignment,child_capability,entry_argv,entry_checker,gate_context,run_epoch,scope_id,task_input_file,task_input_sha256', 'worker message fields mismatch');
  requireDispatch(nonemptyString(received.child_capability) && received.child_capability === scope.child_capability, 'actual child capability absent or changed');
  requireDispatch(received.scope_id === scope.scope_id, 'scope identity changed');
  requireDispatch(args.message === expectedWorkerMessage(scope, seed), 'message differs from the captured inputs');
  requireDispatch(!JSON.stringify(args).includes(scope.parent_capability), 'parent capability included');
  return args;
}
const dispatchGateContext = load('dispatch_gate_context');
requireDispatch(nonemptyString(dispatchGateContext), 'outer gate context missing');
const dispatchRunEpoch = load('dispatch_run_epoch') ?? null;
requireDispatch(dispatchRunEpoch === null || nonemptyString(dispatchRunEpoch), 'invalid run epoch');
const capturedScope = load('focused_scope');
const staticSeed = load('dispatch_seed');
const checked = load('verified_handoff');
requireDispatch(checked && checked.status === 'SCOPE_INPUT_OK', 'durable scope identity has not been verified');
requireDispatch(JSON.stringify(checked.scope) === JSON.stringify(capturedScope), 'scope differs from verified grant');
requireDispatch(JSON.stringify(checked.seed) === JSON.stringify(staticSeed), 'seed differs from exact verified file');
requireDispatch(checked.gate_context === dispatchGateContext && checked.run_epoch === dispatchRunEpoch, 'outer attribution differs from verified scope');
requireDispatch(checked.identity && nonemptyString(checked.identity.task_id) && nonemptyString(checked.identity.change_id) && nonemptyString(checked.identity.phase), 'verified task identity mismatch');
validateSources(capturedScope, staticSeed);
const nativeArguments = {
  task_name: 'focused_worker',
  agent_type: 'docket-build-standard',
  fork_turns: 'none',
  message: expectedWorkerMessage(capturedScope, staticSeed)
};
validateDispatch(nativeArguments, capturedScope, staticSeed);
store('validated_dispatch', nativeArguments);
// This LIVE OPERATIONAL output intentionally contains the actual CHILD token.
// The parent token remains private. Never save this output as public evidence.
text({status: 'DISPATCH_INPUT_OK', dispatch_arguments: nativeArguments});
