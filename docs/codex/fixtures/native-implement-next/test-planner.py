"""Read-only payload mutations against a separate prepared rehearsal worktree."""
from pathlib import Path
import json,subprocess,importlib.util,copy,sys
sys.dont_write_bytecode=True
r=Path(sys.argv[1]).resolve();f=Path(json.loads((r/'worker-inputs.json').read_text())['feature_worktree']);meta=r/'repo/.docket';ch=next((meta/'docs/changes/active').glob('*.md'));sp=next((meta/'docs/superpowers/specs').glob('*.md'))
d={'change_id':1,'title':'Trim surrounding whitespace in greeting','primary_checkout':str(r/'repo'),'feature_worktree':str(f),'feature_ref':subprocess.check_output(['git','-C',str(f),'symbolic-ref','HEAD'],text=True).strip(),'pre_dispatch_head':subprocess.check_output(['git','-C',str(f),'rev-parse','HEAD'],text=True).strip(),'workspace_receipt':str(r/'evidence/workspace-created.json'),'boundary_checker':str(r/'check-boundary.py'),'metadata_worktree':str(meta),'change_file':str(ch),'spec_file':str(sp),'backlink_change_path':str(ch.relative_to(meta)),'plan_skill':'superpowers:writing-plans','plan_skill_file':str(f/'.agents/skills/superpowers-writing-plans/SKILL.md'),'build_skill':'docket-build','build_skill_file':str(f/'.agents/skills/docket-build/SKILL.md'),'learnings_enabled':False,'learnings_index':None,'test_command':'go test -count=1 ./...','build_profile':'standard','task_count':1,'preserve_baseline_tests':True,'assignment':'Write and commit the one-task plan only; synthetic payload validator rehearsal.'}
spec=importlib.util.spec_from_file_location('planner',r/'validate-plan-payload.py');m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);m.validate(d);n=1
for key in d:
 x=copy.deepcopy(d);del x[key]
 try:m.validate(x)
 except (ValueError,KeyError):n+=1
 else:raise AssertionError(key)
for key,value in [('plan_skill_file',str(r/'repo/.agents/skills/superpowers-writing-plans/SKILL.md')),('task_count',2),('feature_worktree',str(r/'repo')),('build_profile','economy'),('preserve_baseline_tests',False)]:
 x=copy.deepcopy(d);x[key]=value
 try:m.validate(x)
 except ValueError:n+=1
 else:raise AssertionError(key)
print(json.dumps({'status':'passed','checks':n,'scope':'synthetic complete planner payload and mutation checks'}))
