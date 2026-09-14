#!/usr/bin/env python3
"""Validate a sanitized acceptance bundle's completeness/consistency, not host authenticity."""
import argparse,hashlib,json,re
from pathlib import Path
SHA=re.compile(r'^[0-9a-f]{40}$')
REQUIRED=('initial','workspace','native','plan','worker','final_gate','results','primary','halt','outer_verdict')
def validate(d,root):
 def need(ok,msg):
  if not ok:raise ValueError(msg)
 need(d.get('schema_version')==1,'unsupported schema')
 need(isinstance(d.get('evidence_gaps'),list),'explicit evidence gaps required')
 for stage in REQUIRED:
  v=d.get(stage);need(isinstance(v,dict),'missing stage '+stage)
  refs=v.get('evidence');need(isinstance(refs,list) and bool(refs),'missing supporting evidence '+stage)
  for x in refs:
   p=Path(x['path']);need(not p.is_absolute() and '..' not in p.parts,'unsafe evidence path')
   f=(root/p).resolve();need(f.is_relative_to(root.resolve()) and f.is_file(),'missing evidence')
   need(hashlib.sha256(f.read_bytes()).hexdigest()==x['sha256'],'evidence hash mismatch')
 need(d['initial'].get('unclaimed') is True and d['initial'].get('plan_absent') is True and d['initial'].get('feature_absent') is True,'initial fixture not fresh')
 n=d['native'];need(n.get('roles')==['docket-implement-next','docket-plan-writer','docket-build-standard'],'wrong native roles')
 need(n.get('single_coordinator') is True and n.get('dispatches')==3 and n.get('terminal_collected') is True,'native lineage or collection incomplete')
 need(n.get('agent_enter_calls')==0,'forbidden entry')
 need(n.get('proof_kind') in ['host-observed','mixed-host-and-child-receipts'],'native proof source missing')
 if n['proof_kind']!='host-observed':need(bool(d['evidence_gaps']),'mixed evidence must disclose gaps')
 w=d['workspace'];need(all(isinstance(w.get(k),str) and Path(w[k]).is_absolute() for k in ['primary','feature']),'missing canonical roots');need(w.get('created_in_run') is True and w.get('registered') is True and w.get('primary')!=w.get('feature'),'invalid worktree creation')
 p=d['plan'];need(p.get('newly_authored') is True and p.get('verified_attached') is True and p.get('task_count')==1,'fresh plan incomplete')
 worker=d['worker'];need(worker.get('consumed_plan_commit')==p.get('commit'),'worker did not consume new plan')
 need(worker.get('outcomes')==['PASSED','FAILED','PASSED'] and worker.get('red_assertion') is True,'TDD incomplete')
 need(worker.get('scope_closed') is True and worker.get('final_acked') is True,'scope acknowledgement absent')
 need(worker.get('changed_files')==['greeting.go','greeting_test.go'],'wrong worker delta')
 need(worker.get('input_hash_valid') is True and worker.get('entry_argv_valid') is True and worker.get('outer_context_preserved') is True,'handoff incomplete')
 need(worker.get('cwd')==w.get('feature') and worker.get('run_root')==worker.get('expected_run_root'),'worker target mismatch')
 for v in [p.get('commit'),worker.get('commit'),d['results'].get('commit'),d['final_gate'].get('head')]:need(isinstance(v,str) and SHA.fullmatch(v),'invalid full commit')
 g=d['final_gate'];need(g.get('command')=='go test -count=1 ./...' and g.get('outcome')=='PASSED' and g.get('clean') is True,'final suite incomplete')
 need(g.get('head')==d['results'].get('commit') and g.get('cwd')==w.get('feature'),'final checkpoint not tested')
 need(d['results'].get('attached') is True and d['results'].get('published_to_local_origin') is True,'results not durable')
 need(d['primary'].get('status')=='PRIMARY_UNCHANGED','primary changed')
 need(d['halt'].get('result')=='applied' and d['halt'].get('disposition')=='halted','typed halt missing')
 need(d['outer_verdict'].get('report')=='gate-stop' and d['outer_verdict'].get('verdict')=='run-halted' and d['outer_verdict'].get('same_key') is True,'outer terminal verdict missing')
 return {'status':'continuous-functional-passed','evidence_audit_complete':not d['evidence_gaps'],'evidence_gaps':d['evidence_gaps'],'limitation':'Consistency of supplied sanitized receipts; authenticity and opaque host behavior require independent review. No manual change423 completion is performed.'}
def main():
 p=argparse.ArgumentParser();p.add_argument('bundle',type=Path);a=p.parse_args()
 try:print(json.dumps(validate(json.loads(a.bundle.read_text()),a.bundle.parent),indent=2))
 except (ValueError,KeyError,TypeError,OSError) as e:print(json.dumps({'status':'certification-incomplete','reason':str(e)}));raise SystemExit(1)
if __name__=='__main__':main()
