import copy,hashlib,importlib.util,json,tempfile
from pathlib import Path
spec=importlib.util.spec_from_file_location('v',Path(__file__).with_name('validate-evidence.py'));m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
with tempfile.TemporaryDirectory(prefix='docket-0423-validator.') as td:
 r=Path(td);(r/'receipt.json').write_text('{"synthetic":true}\n');ref=[{'path':'receipt.json','sha256':hashlib.sha256((r/'receipt.json').read_bytes()).hexdigest()}];sha='a'*40
 d={'schema_version':1,'evidence_gaps':['Synthetic test data; no native run.'],'initial':{'unclaimed':True,'plan_absent':True,'feature_absent':True},'workspace':{'created_in_run':True,'registered':True,'primary':'/p','feature':'/p/.worktrees/f'},'native':{'roles':['docket-implement-next','docket-plan-writer','docket-build-standard'],'single_coordinator':True,'dispatches':3,'terminal_collected':True,'agent_enter_calls':0,'proof_kind':'mixed-host-and-child-receipts'},'plan':{'newly_authored':True,'verified_attached':True,'task_count':1,'commit':sha},'worker':{'consumed_plan_commit':sha,'outcomes':['PASSED','FAILED','PASSED'],'red_assertion':True,'scope_closed':True,'final_acked':True,'changed_files':['greeting.go','greeting_test.go'],'input_hash_valid':True,'entry_argv_valid':True,'outer_context_preserved':True,'cwd':'/p/.worktrees/f','run_root':'/e/w','expected_run_root':'/e/w','commit':sha},'final_gate':{'head':sha,'command':'go test -count=1 ./...','outcome':'PASSED','clean':True,'cwd':'/p/.worktrees/f'},'results':{'commit':sha,'attached':True,'published_to_local_origin':True},'primary':{'status':'PRIMARY_UNCHANGED'},'halt':{'result':'applied','disposition':'halted'},'outer_verdict':{'report':'gate-stop','verdict':'run-halted','same_key':True}}
 for k in m.REQUIRED:d[k]['evidence']=ref
 assert m.validate(d,r)['status']=='continuous-functional-passed';count=1
 def rejects(x):
  global count
  try:m.validate(x,r)
  except (ValueError,KeyError,TypeError):count+=1;return
  raise AssertionError('mutation accepted')
 for stage in m.REQUIRED:
  x=copy.deepcopy(d);del x[stage];rejects(x)
  for key in d[stage]:
   x=copy.deepcopy(d);del x[stage][key];rejects(x)
 for stage,key,value in [('worker','consumed_plan_commit','b'*40),('worker','cwd','/p'),('worker','run_root','/wrong'),('final_gate','head','b'*40),('native','agent_enter_calls',1),('native','roles',['generic']),('initial','plan_absent',False),('worker','outer_context_preserved',False),('worker','outcomes',['PASSED']*3),('worker','changed_files',['AGENTS.md']),('outer_verdict','verdict','run-complete')]:
  x=copy.deepcopy(d);x[stage][key]=value;rejects(x)
 x=copy.deepcopy(d);x['evidence_gaps']=[];rejects(x)
 x=copy.deepcopy(d);x['initial']['evidence'][0]['sha256']='0'*64;rejects(x)
 (r/'receipt.json').write_text('tampered');rejects(d)
 print(json.dumps({'status':'passed','checks':count,'scope':'synthetic acceptance validator mutation tests; no native certification'}))
