#!/usr/bin/env python3
"""Serialize fixed worker inputs from a newly committed plan; no scope or agent."""
import argparse,hashlib,importlib.util,json,re,subprocess,sys
from pathlib import Path
sys.dont_write_bytecode=True
ROOT=Path(__file__).resolve().parent

def main():
 p=argparse.ArgumentParser();p.add_argument('--feature',type=Path,required=True);p.add_argument('--plan-path',required=True);p.add_argument('--routing-reason',required=True);a=p.parse_args();f=a.feature
 assert f.resolve()==f and f.parent==ROOT/'repo/.worktrees'
 def git(*args):return subprocess.check_output(['git','-C',str(f),*args],text=True).strip()
 assert not git('status','--porcelain')
 plan=Path(a.plan_path);assert not plan.is_absolute() and '..' not in plan.parts and plan.parts[:3]==('docs','superpowers','plans')
 assert (f/plan).is_file();assert git('ls-files','--',str(plan))==str(plan)
 source=(f/plan).read_text();assert 'standard' in source.lower() and 'Task 1' in source and 'Task 2' not in source,'plan must have exactly one standard task'
 assert not (ROOT/'worker-inputs.json').exists() and not (ROOT/'dispatch-seed.json').exists(),'fixed input may be issued only once'
 head=git('rev-parse','HEAD');branch=git('branch','--show-current')
 data={'change_id':1,'task_id':'task-1','phase':'build','primary_checkout':str(ROOT/'repo'),'feature_worktree':str(f),'branch':branch,'feature_ref':'refs/heads/'+branch,'pre_dispatch_head':head,'workspace_receipt':str(ROOT/'evidence/workspace-created.json'),'boundary_checker':str(ROOT/'check-boundary.py'),'operation_reference':str(f/'.codex/POC-WORKTREE.md'),'plan_path':str(plan),'plan_file':str(f/plan),'build_profile':'standard','routing_reason':a.routing_reason,'worker_run_root':str(ROOT/'evidence/worker-gates'),'test_command':'go test -count=1 ./...','test_argv':['go','test','-count=1','./...'],'skill_files':{n:str(f/v) for n,v in {'docket-build-task':'.agents/skills/docket-build-task/SKILL.md','docket-convention':'.agents/skills/docket-convention/SKILL.md','docket-build':'.agents/skills/docket-build/SKILL.md','gate-caller-loop':'.agents/skills/docket-build/references/gate-caller-loop.md'}.items()},'assignment':'Implement only Task 1 of the supplied newly authored plan. Preserve plan and baseline cases. Actual docket-build-task baseline, assertion RED, GREEN, one task commit, acknowledgement and structured receipt. No subagents.','input_version':1}
 spec=importlib.util.spec_from_file_location('check',ROOT/'check-task-inputs.py');mod=importlib.util.module_from_spec(spec);spec.loader.exec_module(mod);mod.validate(data,ROOT)
 boundaryspec=importlib.util.spec_from_file_location('boundary',ROOT/'check-boundary.py');boundary=importlib.util.module_from_spec(boundaryspec);boundaryspec.loader.exec_module(boundary);boundary.check(data['primary_checkout'],str(f),data['feature_ref'],head,data['workspace_receipt'],1)
 raw=(json.dumps(data,indent=2)+'\n').encode();digest=hashlib.sha256(raw).hexdigest();seed=json.loads((ROOT/'worker-seed-template.json').read_text());seed.update(task_input_file=str(ROOT/'worker-inputs.json'),task_input_sha256=digest,entry_checker=str(ROOT/'check-task-inputs.py'));seed['entry_argv']=['python3',seed['entry_checker'],'--inputs',seed['task_input_file'],'--sha256',digest];seed['assignment']+=' Preserve the outer gate_context from this native message and pass it unchanged in every scoped test start.'
 scope_spec=importlib.util.spec_from_file_location('scope_check',ROOT/'check-worker-scope.py');scope_mod=importlib.util.module_from_spec(scope_spec);scope_spec.loader.exec_module(scope_mod)
 (ROOT/'scope-identity-args.json').write_text(json.dumps(scope_mod.scope_arguments(data),indent=2)+'\n')
 (ROOT/'worker-inputs.json').write_bytes(raw);(ROOT/'dispatch-seed.json').write_text(json.dumps(seed,indent=2)+'\n')
 print(json.dumps({'status':'WORKER_INPUTS_READY','task_input_sha256':digest,'pre_dispatch_head':head,'entry_argv':seed['entry_argv']},indent=2))
if __name__=='__main__':main()
