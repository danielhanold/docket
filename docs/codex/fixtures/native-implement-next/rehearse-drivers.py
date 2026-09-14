#!/usr/bin/env python3
"""Non-certifying Docket binary rehearsal in this dedicated disposable repository."""
from pathlib import Path
import json,subprocess,hashlib,datetime,os,sys,shlex
ROOT=Path(sys.argv[1]).resolve();REPO=ROOT/'repo';EV=ROOT/'evidence';INPUT=json.loads((ROOT/'worker-inputs.json').read_text());FEATURE=Path(INPUT['feature_worktree'])
ARM=json.loads((ROOT/'rehearsal-private.json').read_text())
CONTEXT=['--gate-context',ARM['dispatch_context']] + (['--run-epoch',ARM['epoch']] if ARM.get('epoch') else [])
records=[]
def shell(argv,cwd=FEATURE):
 p=subprocess.run([str(a) for a in argv],cwd=cwd,text=True,capture_output=True)
 return p.returncode,p.stdout,p.stderr
def safe(d):
 if isinstance(d,dict):return {k:('[REDACTED]' if k in ['parent_capability','child_capability','generation','owner_generation','owner_gen','handoff_token','token','dispatch_context','gate_context','key'] else safe(v)) for k,v in d.items()}
 if isinstance(d,list):return [safe(x) for x in d]
 return d
def save():
 (EV/'binary-operation-receipts.json').write_text(json.dumps(records,indent=2)+'\n')
def require(ok,reason):
 if not ok:raise RuntimeError(reason)
rc,out,err=shell(['docket','capabilities','--json']);c=json.loads(out);require(rc==0 and c['protocol_version']==1 and c['capability_version']==1 and c['result']=='applied','catalog bootstrap failed')
ops={e['id']:e for e in c['commands']};require(len(ops)==len(c['commands']),'duplicate catalog id')
for e in ops.values():require(bool(e['argv']) and set(e['effects'])<={'read','local-write','metadata-write','external-write','process-control'},'unsupported catalog entry')
schemas={}
for name in ['gate.drive.prepare-scope','gate.drive.start','gate.drive.acknowledge','gate.drive.advance','evidence.record','change.halt','change.refresh-claim','context.implementation','artifact.backlink','change.attach-results','run.gate-verdict']:
 rc,out,err=shell(ops['schema']['argv']+['--operation',name,'--json']);s=json.loads(out);require(rc==0 and s['schema_version']==1 and s['protocol_version']==1 and s['result']=='applied','schema failed '+name);schemas[name]=next(x for x in s['operations'] if x['id']==name)
 (EV/('compat-schema-'+name+'.json')).write_text(json.dumps(s,indent=2)+'\n')
def op(name,args=(),repo=FEATURE):
 require(name in ops,'missing operation '+name)
 rc,out,err=shell(ops[name]['argv']+list(args)+['--repo-dir',str(repo),'--json'])
 try:d=json.loads(out)
 except ValueError:raise RuntimeError('non-JSON operation '+name+' exit '+str(rc))
 records.append({'operation':name,'exit_code':rc,'response':safe(d)});save()
 require(d.get('protocol_version')==1 and d.get('operation')==name and d.get('result') in ['applied','no-op'],'operation failed '+name+'; see sanitized receipts')
 return d
config=op('diagnostic.config');require(config['effective']['build']['test_command']['value']==INPUT['test_command'],'configured test differs from input')
state=op('status',['--records']);change=next(x for x in state['changes'] if x['id']==1)
op('change.refresh-claim',['--id','1','--version',change['version']])
workspace=op('workspace.inspect',['--id','1']);require(workspace['path']==str(FEATURE),'workspace mismatch')
# Normal scope grant. No scope or token is shared with the original manual fixture.
identity_args=json.loads((ROOT/'scope-identity-args.json').read_text())
# op supplies repo-dir itself; keep the generated identity array otherwise exact.
require(identity_args[-2:]==['--repo-dir',str(FEATURE)],'scope repo argument mismatch')
grant=op('gate.drive.prepare-scope',identity_args[:-2]+CONTEXT)
require(all(isinstance(grant.get(k),str) and grant[k] for k in ['scope_id','child_capability','parent_capability']),'incomplete grant')
live={'scope':grant,'gate_context':ARM['dispatch_context'],'run_epoch':ARM.get('epoch')}
checked=subprocess.run(['python3',str(ROOT/'check-worker-scope.py')],input=json.dumps(live),cwd=FEATURE,capture_output=True,text=True)
require(checked.returncode==0,'scope predispatch check failed: '+checked.stderr)
verified=json.loads(checked.stdout);require(verified['status']=='SCOPE_INPUT_OK','scope check receipt missing')
(EV/'scope-input-validation.json').write_text(json.dumps({'status':verified['status'],'identity':verified['identity'],'task_input_sha256':verified['seed']['task_input_sha256']},indent=2)+'\n')
# Exercise the actual payload body with the exact checker output, without dispatch.
js="const fs=require('fs'),vm=require('vm');const v=JSON.parse(fs.readFileSync(0,'utf8'));const m={verified_handoff:v,focused_scope:v.scope,dispatch_seed:v.seed,dispatch_gate_context:v.gate_context,dispatch_run_epoch:v.run_epoch};vm.runInNewContext(fs.readFileSync(process.argv[1],'utf8'),{load:k=>m[k],store:(k,v)=>m[k]=v,text:o=>{if(o.status!=='DISPATCH_INPUT_OK')throw Error('payload failed')}});console.log('DISPATCH_INPUT_OK');"
payload=subprocess.run(['node','-e',js,str(ROOT/'dispatch-payload.js')],input=checked.stdout,capture_output=True,text=True)
require(payload.returncode==0 and payload.stdout.strip()=='DISPATCH_INPUT_OK','actual verified grant payload failed')
previous=None
def start_task(label,expected):
 global previous
 args=CONTEXT+['--owner','task','--change-id',str(INPUT['change_id']),'--task-id',INPUT['task_id'],'--phase',INPUT['phase'],'--branch',INPUT['branch'],'--scope-id',grant['scope_id'],'--child-cap',grant['child_capability'],'--run-root',INPUT['worker_run_root'],'--repo-dir',str(FEATURE),'--json']
 if previous:args+=['--predecessor-drive-id',previous['drive_id'],'--predecessor-owner-gen',previous['generation']]
 # Identity/path arguments are loaded from the fixed record; no retyped run root.
 start_argv=ops['gate.drive.start']['argv']+args+['--']+INPUT['test_argv']
 block=(ROOT/'WORKER.md').read_text().split('<!-- drive-capture:start -->\n```sh\n',1)[1].split('\n```',1)[0]
 before=set(Path(INPUT['worker_run_root']).glob('start-response.*'))
 command='worker_run_root='+shlex.quote(INPUT['worker_run_root'])+'\nstart_argv=('+shlex.join(start_argv)+')\n'+block
 rc,out,err=shell(['/bin/zsh','-c',command])
 captures=set(Path(INPUT['worker_run_root']).glob('start-response.*'))-before
 require(len(captures)==1,'first response not preserved')
 capture=captures.pop();require(rc==0,'first response rejected; inspect private '+str(capture))
 require(out==(capture/'stdout.json').read_text(),'response altered after capture')
 rc=int((capture/'exit-code.txt').read_text())
 d=json.loads(out);records.append({'label':label,'operation':'gate.drive.start','exit_code':rc,'response':safe(d)});save()
 require(d.get('result')=='applied' and isinstance(d.get('drive'),dict),'task drive start failed '+label)
 drive=d['drive'];require(drive.get('drive_id') and drive.get('generation'),'drive ownership missing')
 if drive.get('outcome')=='WAITING':
  # Keep recovery authority private for the outer controller; do not start a replacement.
  p=ROOT/'pending-drive-private.json';p.write_text(json.dumps({'scope':grant,'drive':drive}));p.chmod(0o600)
  raise RuntimeError('WAITING: same drive must be collected using retained private receipt')
 require(drive.get('outcome')==expected,label+' unexpected disposition')
 require(drive.get('run_root')==INPUT['worker_run_root'],label+' wrong run root')
 previous=drive
 return drive
baseline=start_task('baseline','PASSED')
test=FEATURE/'greeting_test.go';text=test.read_text();needle='\t\t{"Ada Lovelace", "Hello, Ada Lovelace!"},';require(needle in text,'baseline fixture changed')
extra='\n'.join(['\t\t{"  Ada  ", "Hello, Ada!"},','\t\t{"\\tAda\\n", "Hello, Ada!"},','\t\t{"\\u00a0Ada\\u00a0", "Hello, Ada!"},','\t\t{" \\t\\n", "Hello!"},','\t\t{"\\u00a0\\u2003", "Hello!"},','\t\t{"  Ada  Lovelace  ", "Hello, Ada  Lovelace!"},'])
test.write_text(text.replace(needle,needle+'\n'+extra))
red=start_task('assertion-red','FAILED')
# Find the durable failed run by its supervisor record, without inspecting capability storage.
failed_logs=[p for p in Path(INPUT['worker_run_root']).glob('*/stdout.log') if '--- FAIL: TestGreetBaseline' in p.read_text()]
require(len(failed_logs)==1 and 'Greet(' in failed_logs[0].read_text(),'RED was not the expected assertion failure')
source=FEATURE/'greeting.go';text=source.read_text();require('strings.TrimSpace' not in text,'implementation already present')
source.write_text(text.replace('package greeting\n','package greeting\n\nimport "strings"\n',1).replace('func Greet(name string) string {','func Greet(name string) string {\n\tname = strings.TrimSpace(name)',1))
green=start_task('green','PASSED')
rc,out,err=shell(['git','diff','--check']);require(rc==0,'diff check failed')
rc,out,err=shell(['git','add','--','greeting.go','greeting_test.go']);require(rc==0,'stage failed')
rc,out,err=shell(['git','commit','-m','Compatibility rehearsal: trim greeting whitespace']);require(rc==0,'commit failed')
rc,head,err=shell(['git','rev-parse','HEAD']);head=head.strip();require(rc==0,'HEAD read failed')
ack=op('gate.drive.acknowledge',['--scope-id',grant['scope_id'],'--child-cap',grant['child_capability'],'--drive-id',green['drive_id'],'--owner-gen',green['generation']])
rc,status,err=shell(['git','status','--porcelain']);require(rc==0 and not status,'feature not clean')
final=op('gate.drive.start',['--owner','build','--change-id','1','--branch',INPUT['branch'],'--cwd',str(FEATURE),'--run-root',str(EV/'final-build-gates')]+CONTEXT)
# Build-owned drive has its normal operation envelope; inspect the actual shape, not a copied one.
drive=final.get('drive',final)
if drive.get('outcome')=='WAITING':
 p=ROOT/'pending-build-private.json';p.write_text(json.dumps(drive));p.chmod(0o600);raise RuntimeError('WAITING final build: collect retained same drive')
require(drive.get('outcome')=='PASSED','final build did not pass')
run_dir=drive.get('raw_run_dir');require(bool(run_dir),'final build missing evidence dir')
evidence=op('evidence.record',['--id','1','--head',head,'--run',run_dir]);require(evidence.get('outcome')=='green' and evidence.get('head')==head,'evidence head mismatch')
rc,current,err=shell(['git','rev-parse','HEAD']);rc2,status,err=shell(['git','status','--porcelain']);require(current.strip()==head and rc2==0 and not status,'final test HEAD/tree changed')
implementation_head=head
results_path='docs/results/2026-09-14-nonnative-rehearsal-results.md'
result_file=FEATURE/results_path;result_file.parent.mkdir(parents=True,exist_ok=True)
result_file.write_text('# Non-native backend rehearsal results\n\n## Outcome\n\nKnown greeting fix completed in a separate disposable rehearsal; no native certification.\n\n## Verification performed\n\nBaseline, genuine RED, GREEN, commit/ack and implementation suite passed. The final checkpoint is tested separately after this commit.\n\n## Findings and limitations\n\nNo native agents, review, PR or merge. This plan and implementation were synthetic backend rehearsal only.\n')
state=op('status');change=next(x for x in state['changes'] if x['id']==1)
op('artifact.backlink',['--artifact',results_path,'--change',change['path']])
rc,out,err=shell(['git','add','--',results_path]);require(rc==0,'results stage failed')
rc,out,err=shell(['git','commit','-m','Record non-native rehearsal results']);require(rc==0,'results commit failed')
rc,out,err=shell(['git','push','origin',INPUT['branch']]);require(rc==0,'local origin publication failed')
rc,head,err=shell(['git','rev-parse','HEAD']);head=head.strip()
state=op('status');change=next(x for x in state['changes'] if x['id']==1)
op('change.attach-results',['--id','1','--version',change['version'],'--path',results_path,'--commit',head])
last=op('gate.drive.start',['--owner','build','--change-id','1','--branch',INPUT['branch'],'--cwd',str(FEATURE),'--run-root',str(EV/'final-build-gates')]+CONTEXT)
lastdrive=last.get('drive',last)
if lastdrive.get('outcome')=='WAITING':
 pending=ROOT/'pending-checkpoint-private.json';pending.write_text(json.dumps(lastdrive));pending.chmod(0o600);raise RuntimeError('WAITING checkpoint: collect retained same drive')
require(lastdrive.get('outcome')=='PASSED','checkpoint gate not passed')
checkpoint_evidence=op('evidence.record',['--id','1','--head',head,'--run',lastdrive['raw_run_dir']]);require(checkpoint_evidence.get('outcome')=='green' and checkpoint_evidence.get('head')==head,'checkpoint evidence mismatch')
state=op('status',['--records']);change=next(x for x in state['changes'] if x['id']==1)
request={'report':'Completed non-certifying binary compatibility rehearsal: fixed input baseline, assertion RED, GREEN, task commit and scope acknowledgement, clean configured final suite and exact-HEAD evidence. No native agents were used. Original manual fixture was not executed.'}
fields=schemas['change.halt']['request']['fields'];require('report' in {x['key'] for x in fields},'halt schema lacks report')
p=EV/'compat-halt-input.json';p.write_text(json.dumps(request)+'\n')
halt=op('change.halt',['--id','1','--version',change['version'],'--input',str(p)]);require(halt.get('disposition')=='halted','halt incomplete')
rc,out,err=shell(['python3',str(ROOT/'primary-audit.py'),'check','--repo',str(REPO),'--baseline',str(EV/'primary-baseline.json')],REPO);audit=json.loads(out);require(rc==0 and audit['status']=='PRIMARY_UNCHANGED','primary audit failed')
verdict=op('run.gate-verdict',[ARM['key']],repo=REPO);require(verdict.get('decision')=='gate-stop' and verdict.get('outcome')=='run-halted','outer verdict mismatch')
report={'status':'passed','scope':'full Docket binary compatibility rehearsal; no native-agent certification','binary':c['binary'],'tested_clean_implementation_head':implementation_head,'final_checkpoint_head':head,'checkpoint_evidence':checkpoint_evidence,'outer_verdict':safe(verdict),'epoch_supplied':bool(ARM.get('epoch')),'baseline':'PASSED','red':'genuine assertion FAILED','green':'PASSED','final_scope_acknowledgement':'applied','configured_full_suite':evidence,'halt':halt,'primary_audit':audit,'source_paths':{'repo':str(REPO),'feature':str(FEATURE)},'original_manual_fixture_untouched':True,'completed_at':datetime.datetime.now(datetime.timezone.utc).isoformat()}
(EV/'binary-compatibility.json').write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps({'status':report['status'],'binary':c['binary']['version'],'baseline':'PASSED','red':'assertion FAILED','green':'PASSED','acknowledgement':'applied','final_suite':'PASSED','clean_head':head,'halt':'halted','primary':audit['status']},indent=2))
