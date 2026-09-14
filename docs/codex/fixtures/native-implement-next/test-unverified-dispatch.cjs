const fs=require('fs'),vm=require('vm');
const code=fs.readFileSync(require('path').join(__dirname,'template/dispatch-payload.js'),'utf8');
const seed={task_input_file:'/fixture/worker-inputs.json',task_input_sha256:'a'.repeat(64),entry_checker:'/fixture/check-task-inputs.py',assignment:'Real task'};
seed.entry_argv=['python3',seed.entry_checker,'--inputs',seed.task_input_file,'--sha256',seed.task_input_sha256];
const scope={scope_id:'scope',child_capability:'child',parent_capability:'parent'};
const mem={dispatch_seed:seed,focused_scope:scope,dispatch_gate_context:'context',dispatch_run_epoch:'epoch'};
let rejected=false;
try { vm.runInNewContext(code,{load:k=>mem[k],store:(k,v)=>mem[k]=v,text:()=>{}}); }
catch(e){if(!e.message.startsWith('DISPATCH_INPUT_FAIL:')) throw e; rejected=true;}
if(!rejected)throw Error('REGRESSION: native dispatch accepted a scope without checking its task identity against the fixed input');
console.log('PASS: unverified scope refused before dispatch');
const good={status:'SCOPE_INPUT_OK',scope,seed,gate_context:'context',run_epoch:'epoch',identity:{task_id:'task-1',change_id:'1',phase:'build'}};
function execute(proof){const memory={...mem,verified_handoff:proof};vm.runInNewContext(code,{load:k=>memory[k],store:(k,v)=>memory[k]=v,text:()=>{}});}
execute(good);
for(const [field,value] of [['status','FAIL'],['scope',{...scope,scope_id:'other'}],['seed',{...seed,assignment:'transcribed assignment'}],['gate_context','other'],['run_epoch','other'],['identity',{}]]){
 let rejected=false;try{execute({...good,[field]:value});}catch(e){if(!e.message.startsWith('DISPATCH_INPUT_FAIL:'))throw e;rejected=true;}
 if(!rejected)throw Error('Changed verified '+field+' accepted');
}
console.log('PASS: verified payload accepted; six changed proof fields refused');
