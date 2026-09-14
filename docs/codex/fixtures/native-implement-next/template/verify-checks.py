#!/usr/bin/env python3
"""Mutation checks for read-only preconditions. No Docket operations or agent/test gates."""
import copy,hashlib,importlib.util,json,subprocess,sys,tempfile
from pathlib import Path
sys.dont_write_bytecode=True
ROOT=Path(__file__).resolve().parent

def module(name,file):
 s=importlib.util.spec_from_file_location(name,ROOT/file);m=importlib.util.module_from_spec(s);s.loader.exec_module(m);return m
inputs=module('task_inputs','check-task-inputs.py');runtime=module('runtime','check-runtime.py')
path=ROOT/'worker-inputs.json';raw=path.read_bytes();data=json.loads(raw);digest=hashlib.sha256(raw).hexdigest();checks=[]
def reject(label,fn):
 try:fn()
 except (ValueError,OSError,KeyError,TypeError):checks.append(label+': rejected')
 else:raise AssertionError(label+' incorrectly passed')
assert inputs.load_checked(path,digest)==data;checks.append('valid fixed input accepted')
for key in data:
 bad=copy.deepcopy(data);del bad[key];reject('missing input '+key,lambda bad=bad:inputs.validate(bad,ROOT))
for key,value in [('worker_run_root','/invalid-shortened-fixture/worker-gates'),('phase','other'),('test_argv',['echo','pretend pass']),('feature_worktree',data['primary_checkout']),('feature_ref','refs/heads/wrong'),('workspace_receipt',str(ROOT/'evidence/other.json')),('parent_capability','synthetic forbidden value')]:
 bad=copy.deepcopy(data);bad[key]=value;reject('mutated '+key,lambda bad=bad:inputs.validate(bad,ROOT))
bad=copy.deepcopy(data);bad['skill_files']['docket-build-task']=str(ROOT/'repo/.agents/skills/docket-build-task/SKILL.md');reject('primary skill path',lambda:inputs.validate(bad,ROOT))
reject('wrong input hash',lambda:inputs.load_checked(path,'0'*64))
try:
 path.write_bytes(raw+b'\n');reject('input bytes modified',lambda:inputs.load_checked(path,digest))
finally:path.write_bytes(raw)
assert inputs.load_checked(path,digest)==data;checks.append('restored input accepted')
tid='00000000-0000-0000-0000-000000000001';primary=data['primary_checkout']
records=[{'type':'session_meta','payload':{'id':tid,'cwd':primary,'source':'cli','cli_version':'synthetic'}},{'type':'turn_context','timestamp':'2026-09-14T12:00:00Z','payload':{'cwd':primary,'model':'gpt-5.6-terra','effort':'low'}}]
assert runtime.validate(records,tid,primary)['status']=='RUNTIME_OK';checks.append('synthetic correct runtime accepted')
for field,value in [('model','gpt-5.6-luna'),('effort','medium'),('cwd','/wrong')]:
 bad=copy.deepcopy(records);bad[-1]['payload'][field]=value;reject('runtime '+field,lambda bad=bad:runtime.validate(bad,tid,primary))
reject('missing runtime context',lambda:runtime.validate(records[:1],tid,primary))
reject('wrong task identity',lambda:runtime.validate(records,'other',primary))
bad=copy.deepcopy(records);bad[0]['payload']['source']={'subagent':{}};reject('nested controller',lambda:runtime.validate(bad,tid,primary))
bad=copy.deepcopy(records);bad.append({'type':'turn_context','timestamp':'2026-09-14T13:00:00Z','payload':{'cwd':primary,'model':'gpt-5.6-terra','effort':'medium'}});reject('earlier correct effort cannot mask latest wrong effort',lambda:runtime.validate(bad,tid,primary))
# Ordinary shell/JQ argument loading is tested with capture only, never a driver invocation.
script='''task_inputs="$1"
task_json=$(cat "$task_inputs") || exit 1
worker_run_root=$(jq -er '.worker_run_root' <<< "$task_json") || exit 1
feature_root=$(jq -er '.feature_worktree' <<< "$task_json") || exit 1
task_branch=$(jq -er '.branch' <<< "$task_json") || exit 1
task_change=$(jq -er '.change_id' <<< "$task_json") || exit 1
task_name=$(jq -er '.task_id' <<< "$task_json") || exit 1
task_phase=$(jq -er '.phase' <<< "$task_json") || exit 1
test_args=()
while IFS= read -r test_arg; do test_args+=("$test_arg"); done < <(jq -er '.test_argv[]' <<< "$task_json")
printf '%s\\n' "$worker_run_root" "$feature_root" "$task_branch" "$task_change" "$task_name" "$task_phase" "${test_args[@]}"
'''
for shell in ['/bin/zsh','/bin/bash']:
 result=subprocess.check_output([shell,'-c',script,'capture-only',str(path)],text=True)
 assert result.splitlines()==[data['worker_run_root'],data['feature_worktree'],data['branch'],str(data['change_id']),data['task_id'],data['phase'],*data['test_argv']]
 checks.append(shell+' loads exact JSON values and preserves argument boundaries')
 with tempfile.TemporaryDirectory(prefix='quoted-input.',dir=ROOT) as temp:
  p=Path(temp)/'task inputs.json';other=data|{'worker_run_root':str(Path(temp)/'spaces ; literal $value')};p.write_text(json.dumps(other))
  lines=subprocess.check_output([shell,'-c',script,'capture-only',str(p)],text=True).splitlines();assert lines[0]==other['worker_run_root'];checks.append(shell+' quoted path is read literally without evaluation')
result={'status':'passed','check_count':len(checks),'checks':checks,'scope':'precondition and input-loading verification only; no native run, task scopes or test drivers launched'}
(ROOT/'evidence/check-validation.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps({k:result[k] for k in ['status','check_count','scope']},indent=2))
