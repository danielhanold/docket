// Exercise the actual prepared payload-to-entry boundary without any agent or gate.
const fs=require('fs');
const path=require('path');
const vm=require('vm');
const cp=require('child_process');
const root=__dirname;
const seed=JSON.parse(fs.readFileSync(path.join(root,'dispatch-seed.json'),'utf8'));
const inputs=JSON.parse(fs.readFileSync(seed.task_input_file,'utf8'));
const scope={scope_id:'rehearsal-scope',child_capability:'rehearsal-child',parent_capability:'rehearsal-private-parent'};
const memory={dispatch_run_epoch:"synthetic-epoch",dispatch_gate_context:"synthetic-outer-context",focused_scope:scope,dispatch_seed:seed};const outputs=[];
memory.verified_handoff={status:'SCOPE_INPUT_OK',scope,seed,gate_context:memory.dispatch_gate_context,run_epoch:memory.dispatch_run_epoch,identity:{task_id:inputs.task_id,change_id:String(inputs.change_id),phase:inputs.phase}};
const context=vm.createContext({load:k=>memory[k],store:(k,v)=>{memory[k]=v;},text:v=>outputs.push(v)});
vm.runInContext(fs.readFileSync(path.join(root,'dispatch-payload.js'),'utf8'),context);
if(outputs.length!==1 || outputs[0].status!=='DISPATCH_INPUT_OK')throw Error('payload not validated');
const args=outputs[0].dispatch_arguments;
const workerMessage=JSON.parse(args.message);
if(workerMessage.child_capability!==scope.child_capability || JSON.stringify(args).includes(scope.parent_capability))throw Error('invalid credential handoff');
if(JSON.stringify(workerMessage.entry_argv)!==JSON.stringify(seed.entry_argv))throw Error('entry command changed across message');
function quote(x){return "'"+x.replace(/'/g,"'\\''")+"'";}
function run(argv){
 const command=argv.map(quote).join(' ');
 const result=cp.spawnSync('/bin/zsh',['-c',command],{cwd:inputs.primary_checkout,encoding:'utf8'});
 if(result.error)throw result.error;
 return {command,exit_code:result.status,stdout:result.stdout,stderr:result.stderr};
}
const exact=run(workerMessage.entry_argv);
if(exact.exit_code!==0 || JSON.parse(exact.stdout).status!=='BINDING_OK')throw Error('exact payload entry failed: '+JSON.stringify(exact));
const direct=run(workerMessage.entry_argv.slice(1));
if(direct.exit_code!==0 || JSON.parse(direct.stdout).status!=='BINDING_OK')throw Error('direct entry fallback failed');
const checker=workerMessage.entry_checker;const originalMode=fs.statSync(checker).mode&0o777;
if(originalMode!==0o755)throw Error('checker must be 0755');
let blocked;
try{
 fs.chmodSync(checker,0o644);
 blocked=run(workerMessage.entry_argv.slice(1));
 if(blocked.exit_code!==126)throw Error('permission regression did not reproduce');
 if((fs.statSync(checker).mode&0o777)===originalMode)throw Error('mode mutation not detected');
}finally{fs.chmodSync(checker,originalMode);}
const restored=run(workerMessage.entry_argv);
if(restored.exit_code!==0)throw Error('restored entry failed');
const report={status:'passed',scope:'actual static payload and entry command with synthetic credentials; no native dispatch or test gate',payload_roundtrip:{actual_child_value_retained:true,parent_value_absent:true,entry_argv_unchanged:true},exact_payload_command:exact,direct_executable_check:direct,permission_regression:{nonexecutable_direct_exit:blocked.exit_code,mode_difference_detected:true,restored_mode:'0755',restored_exit:restored.exit_code},primary_cwd:inputs.primary_checkout};
fs.writeFileSync(path.join(root,'evidence/entry-rehearsal.json'),JSON.stringify(report,null,2)+'\n');
console.log(JSON.stringify({status:report.status,exact_payload_exit:exact.exit_code,direct_exit:direct.exit_code,permission_mutation_exit:blocked.exit_code,restored_exit:restored.exit_code,scope:report.scope}));
