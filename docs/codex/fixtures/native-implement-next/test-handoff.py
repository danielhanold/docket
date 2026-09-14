#!/usr/bin/env python3
"""Focused regression: pinned scope identity and exact worker response capture."""
import copy,hashlib,importlib.util,json,shlex,subprocess,sys,tempfile
from pathlib import Path
sys.dont_write_bytecode=True
PACKAGE=Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('scope_check',PACKAGE/'template/check-worker-scope.py');scope=importlib.util.module_from_spec(spec);spec.loader.exec_module(scope)
checks=[]
def rejects(label,fn):
 try:fn()
 except ValueError:checks.append(label);return
 raise AssertionError('guard accepted '+label)
data={'change_id':1,'task_id':'task-1','phase':'build','branch':'feat/example','feature_worktree':'/fixture/repo/.worktrees/example'}
grant={'child_capability':'synthetic-child','parent_capability':'synthetic-parent'}
record={'schema_version':3,'repo_identity':'/fixture/repo/.git','change_id':'1','task_id':'task-1','phase':'build','branch':data['branch'],'worktree':data['feature_worktree'],'gate_context_hash':scope.digest('context'),'run_epoch_id':'epoch','child_cap_hash':scope.digest(grant['child_capability']),'parent_cap_hash':scope.digest(grant['parent_capability']),'drive_count':0,'closed':False}
def verify(r):return scope.verify_record(data,r,grant,'context','epoch',Path('/fixture/repo/.git'))
verify(record);checks.append('matching unused scope accepted')
for k,bad in [('task_id','1'),('change_id','2'),('phase','plan'),('branch','main'),('worktree','/fixture/repo'),('repo_identity','/other/.git'),('gate_context_hash','bad'),('run_epoch_id','other'),('child_cap_hash','bad'),('parent_cap_hash','bad'),('schema_version',4),('drive_count',1),('closed',True),('current_drive_id','drive'),('final_acked',True)]:
 changed=copy.deepcopy(record);changed[k]=bad;rejects('changed '+k,lambda:verify(changed))
for k in ['task_id','change_id','phase','branch','worktree','repo_identity','gate_context_hash','run_epoch_id','child_cap_hash','parent_cap_hash']:
 changed=copy.deepcopy(record);del changed[k];rejects('missing '+k,lambda:verify(changed))
args=scope.scope_arguments(data);assert args[args.index('--task-id')+1]=='task-1';checks.append('scope task identity generated from fixed input')
# Execute the exact maintained worker shell recipe with controllable subprocess output.
source=(PACKAGE/'template/WORKER.md').read_text();block=source.split('<!-- drive-capture:start -->\n```sh\n',1)[1].split('\n```',1)[0]
with tempfile.TemporaryDirectory(prefix='docket-423-receipts-') as tmp:
 root=Path(tmp);worker=root/'worker-gates'
 def run_case(label,response,rc,expected):
  before=set(worker.glob('start-response.*'))
  script='import sys;sys.stdout.write('+repr(response)+');sys.stderr.write("original stderr\\n");sys.exit('+str(rc)+')'
  init='worker_run_root='+shlex.quote(str(worker))+'\nstart_argv=('+shlex.join([sys.executable,'-c',script])+')\n'
  p=subprocess.run(['/bin/zsh','-c',init+block],capture_output=True,text=True)
  assert (p.returncode==0)==expected,(label,p.stdout,p.stderr)
  captures=set(worker.glob('start-response.*'))-before;assert len(captures)==1
  capture=captures.pop();assert (capture/'stdout.json').read_text()==response and (capture/'stderr.txt').read_text()=='original stderr\n'
  assert int((capture/'exit-code.txt').read_text())==rc
  assert (capture/'stdout.json').stat().st_mode&0o077==0
  checks.append(label+'; original stdout/stderr/exit retained privately')
 for outcome,rc in [('PASSED',0),('FAILED',1),('WAITING',0),('HALTED',1)]:
  response={'protocol_version':1,'operation':'gate.drive.start','result':'applied','drive':{'drive_id':'drive','generation':'owner','outcome':outcome,'run_root':str(worker)}}
  run_case('nested '+outcome,json.dumps(response),rc,True)
 run_case('scope mismatch exit2',json.dumps({'protocol_version':1,'operation':'gate.drive.start','result':'invalid-input','reason':'scope-identity-mismatch'}),2,False)
 run_case('non-JSON error','raw error\n',2,False)
 run_case('empty output','',2,False)
 flat={'protocol_version':1,'operation':'gate.drive.start','result':'applied','drive_id':'drive','owner_generation':'owner','outcome':'PASSED','run_root':str(worker)}
 run_case('wrong top-level receipt shape',json.dumps(flat),0,False)
 good={'protocol_version':1,'operation':'gate.drive.start','result':'applied','drive':{'drive_id':'drive','generation':'owner','outcome':'PASSED','run_root':str(worker)}}
 for key in ['drive_id','generation','outcome','run_root']:
  mutated=copy.deepcopy(good);del mutated['drive'][key];run_case('missing nested '+key,json.dumps(mutated),0,False)
 mutated=copy.deepcopy(good);mutated['drive']['run_root']='/wrong';run_case('wrong run root',json.dumps(mutated),0,False)
print(json.dumps({'status':'passed','check_count':len(checks),'checks':checks,'scope':'synthetic scope and exact shell recipe; no native certification'},indent=2))
