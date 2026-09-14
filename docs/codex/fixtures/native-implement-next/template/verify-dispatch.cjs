// Synthetic mutation tests only. This process cannot dispatch agents or run gates.
const fs = require('fs');
const path = require('path');
const vm = require('vm');
const root = __dirname;
const code = fs.readFileSync(path.join(root, 'dispatch-payload.js'), 'utf8');
const scope = {scope_id:'synthetic-scope',child_capability:'synthetic-child-token',parent_capability:'synthetic-parent-token'};
const seed = {task_input_file:'/fixture/worker-inputs.json',task_input_sha256:'a'.repeat(64),entry_checker:'/fixture/check-task-inputs.py',assignment:'Carry out the real one-task worker charter.'};
seed.entry_argv=['python3',seed.entry_checker,'--inputs',seed.task_input_file,'--sha256',seed.task_input_sha256];
const checks = [];
function execute(sc=scope, sd=seed, gate="synthetic-outer-context", epoch="synthetic-epoch") {
  const memory = {dispatch_run_epoch:epoch,dispatch_gate_context:gate,focused_scope:sc,dispatch_seed:sd};
  memory.verified_handoff={status:'SCOPE_INPUT_OK',scope:sc,seed:sd,gate_context:gate,run_epoch:epoch,identity:{task_id:'synthetic-task',change_id:'1',phase:'build'}};
  const emitted = [];
  const context = vm.createContext({load:k=>memory[k],store:(k,v)=>{memory[k]=v;},text:v=>emitted.push(v)});
  vm.runInContext(code, context);
  return {context,memory,emitted};
}
function rejected(label, fn) {
  let failed=false;
  try { fn(); } catch(e) { if(!e.message.startsWith('DISPATCH_INPUT_FAIL:')) throw e; failed=true; }
  if(!failed) throw Error(label+' unexpectedly passed');
  checks.push(label+': rejected');
}
for (const entry_argv of [undefined,[],[seed.entry_checker],['python3',seed.entry_checker],['python3','/wrong','--inputs',seed.task_input_file,'--sha256',seed.task_input_sha256]]) rejected('missing/changed entry argv',()=>execute(scope,{...seed,entry_argv}));
const result = execute();
const good = JSON.parse(JSON.stringify(result.memory.validated_dispatch));
if(result.emitted.length!==1 || result.emitted[0].status!=='DISPATCH_INPUT_OK') throw Error('missing ready output');
if(JSON.stringify(result.emitted[0].dispatch_arguments)!==JSON.stringify(good)) throw Error('emitted and stored payload differ');
const msg=JSON.parse(good.message);
if(msg.child_capability!==scope.child_capability || JSON.stringify(result.emitted).includes(scope.parent_capability)) throw Error('incorrect credential exposure');
checks.push('actual child token exported; parent token excluded; stored and emitted arguments identical');
const validate = result.context.validateDispatch;
for(const key of ['scope_id','child_capability','parent_capability']) {
 const altered={...scope};delete altered[key];rejected('missing captured '+key,()=>execute(altered));
}
for(const bad of [undefined,null,true,false,'','   ','wrong-token','[REDACTED]']) {
 const args={...good};const body=JSON.parse(args.message);
 if(bad===undefined) delete body.child_capability;else body.child_capability=bad;
 args.message=JSON.stringify(body,null,2);
 rejected('missing/invalid/changed child token '+String(bad),()=>validate(args,scope,seed));
}
const marker={...good};const body=JSON.parse(marker.message);delete body.child_capability;body.child_capability_present=true;marker.message=JSON.stringify(body,null,2);
rejected('presence flag replacing actual child token',()=>validate(marker,scope,seed));
for(const [key,value] of [['scope_id','wrong-scope'],['parent_capability',scope.parent_capability],['assignment','Leaked '+scope.parent_capability],['task_input_sha256','b'.repeat(64)]]) {
 const args={...good};const body=JSON.parse(args.message);body[key]=value;args.message=JSON.stringify(body,null,2);
 rejected('changed message '+key,()=>validate(args,scope,seed));
}
rejected('parent token hidden in static seed',()=>execute(scope,{...seed,assignment:scope.parent_capability}));
rejected('equal parent and child token',()=>execute({...scope,parent_capability:scope.child_capability}));
for(const [key,value] of [['agent_type','generic-worker'],['fork_turns','all'],['task_name','other']]) rejected('wrong native '+key,()=>validate({...good,[key]:value},scope,seed));
rejected('extra native argument',()=>validate({...good,parent_capability:scope.parent_capability},scope,seed));
rejected('missing scope memory',()=>execute(null));
rejected('missing seed memory',()=>execute(scope,null));
const fixtureSeed=JSON.parse(fs.readFileSync(path.join(root,'dispatch-seed.json'),'utf8'));
const fixtureRun=execute(scope,fixtureSeed);
if(JSON.parse(fixtureRun.memory.validated_dispatch.message).task_input_sha256!==fixtureSeed.task_input_sha256) throw Error('fixture digest changed');
checks.push('actual fixture seed exports complete authorized child payload with synthetic scope');
for(const gate of [null,true,'','   ']) rejected('outer context missing or invalid',()=>execute(scope,seed,gate));
const changed={...good};const changedBody=JSON.parse(changed.message);changedBody.gate_context='wrong-context';changed.message=JSON.stringify(changedBody,null,2);rejected('outer context changed',()=>validate(changed,scope,seed));
for(const epoch of [true,'','   ']) rejected('invalid run epoch',()=>execute(scope,seed,'synthetic-outer-context',epoch));
execute(scope,seed,'synthetic-outer-context',null);checks.push('optional epoch absence accepted');
execute();checks.push('valid payload accepted again after mutations');
const report={status:'passed',check_count:checks.length,checks,scope:'synthetic token and payload validation; no agents, real capabilities or gate scopes'};
fs.writeFileSync(path.join(root,'evidence/dispatch-validation.json'),JSON.stringify(report,null,2)+'\n');
console.log(JSON.stringify({status:report.status,check_count:report.check_count,scope:report.scope}));
