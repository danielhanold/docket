#!/usr/bin/env python3
"""Controller-only, read-only identity check. No Docket calls or dispatch.

The pinned binary exposes no scope-inspection operation. This POC checker reads
its v3 record without modifying it; unknown schemas fail closed. This is fixture
validation, not a production scope API. Output is PRIVATE operational data.
"""
import hashlib,json,re,subprocess,sys
from pathlib import Path
sys.dont_write_bytecode=True
ROOT=Path(__file__).resolve().parent

def require(ok,reason):
 if not ok:raise ValueError('SCOPE_INPUT_FAIL: '+reason)
def digest(value):return hashlib.sha256(value.encode()).hexdigest()
def scope_arguments(data):
 return ['--change-id',str(data['change_id']),'--task-id',data['task_id'],'--phase',data['phase'],'--branch',data['branch'],'--worktree',data['feature_worktree'],'--repo-dir',data['feature_worktree']]
def verify_record(data,record,grant,context,epoch,common):
 require(record.get('schema_version')==3,'unsupported scope schema')
 expected={'repo_identity':str(common),'change_id':str(data['change_id']),'task_id':data['task_id'],'phase':data['phase'],'branch':data['branch'],'worktree':data['feature_worktree'],'gate_context_hash':digest(context),'run_epoch_id':epoch or ''}
 for k,v in expected.items():require(record.get(k,'')==v,'scope identity mismatch: '+k)
 for key in ['child','parent']:require(record.get(key+'_cap_hash')==digest(grant[key+'_capability']),key+' capability mismatch')
 require(grant['child_capability']!=grant['parent_capability'],'capabilities must differ')
 require(record.get('drive_count')==0 and record.get('closed') is False and not record.get('current_drive_id') and not record.get('final_acked'),'scope already used')
 return expected

def main():
 live=json.load(sys.stdin);grant=live['scope'];context=live['gate_context'];epoch=live.get('run_epoch')
 require(isinstance(context,str) and bool(context.strip()),'outer context missing')
 require(epoch is None or isinstance(epoch,str) and bool(epoch.strip()),'invalid epoch')
 require(grant.get('operation')=='gate.drive.prepare-scope' and grant.get('protocol_version')==1 and grant.get('result')=='applied','scope response envelope')
 for k in ['scope_id','child_capability','parent_capability']:require(isinstance(grant.get(k),str) and bool(grant[k]),'missing '+k)
 require(re.fullmatch('[a-f0-9]+',grant['scope_id']) is not None,'invalid scope locator')
 seed=json.loads((ROOT/'dispatch-seed.json').read_text());raw=(ROOT/'worker-inputs.json').read_bytes();data=json.loads(raw)
 require(seed['task_input_file']==str(ROOT/'worker-inputs.json'),'input locator')
 require(hashlib.sha256(raw).hexdigest()==seed['task_input_sha256'],'input hash')
 require(seed['entry_checker']==str(ROOT/'check-task-inputs.py'),'entry checker')
 require(seed['entry_argv']==['python3',seed['entry_checker'],'--inputs',seed['task_input_file'],'--sha256',seed['task_input_sha256']],'entry argv')
 require(json.loads((ROOT/'scope-identity-args.json').read_text())==scope_arguments(data),'scope arguments changed')
 feature=Path(data['feature_worktree']);common=Path(subprocess.check_output(['git','-C',str(feature),'rev-parse','--path-format=absolute','--git-common-dir'],text=True).strip()).resolve()
 require(common==ROOT/'repo/.git','unexpected common directory')
 path=common/'docket/gate-scopes/v1'/grant['scope_id']/'record.json'
 require(path.resolve()==path,'scope path redirected')
 identity=verify_record(data,json.loads(path.read_text())['record'],grant,context,epoch,common)
 print(json.dumps({'status':'SCOPE_INPUT_OK','scope':grant,'seed':seed,'gate_context':context,'run_epoch':epoch,'identity':identity}))
if __name__=='__main__':
 try:main()
 except (ValueError,KeyError,OSError) as exc:sys.exit(str(exc))
