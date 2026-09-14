"""Non-native rehearsal in a separate copy; never a certification result."""
import json,subprocess,sys
from pathlib import Path
r=Path(sys.argv[1]).resolve();repo=r/'repo';ev=r/'evidence'
def run(argv,cwd=repo):
 p=subprocess.run([str(a) for a in argv],cwd=cwd,text=True,capture_output=True)
 if p.returncode:raise RuntimeError(p.stdout+'\n'+p.stderr)
 return p.stdout.strip()
c=json.loads(run(['docket','capabilities','--json']));assert c['protocol_version']==1 and c['capability_version']==1;ops={x['id']:x for x in c['commands']}
for name in ['run.gate-before','run.gate-verdict','change.claim','change.reconcile','workspace.prepare','artifact.backlink','change.attach-plan']:
 d=json.loads(run(ops['schema']['argv']+['--operation',name,'--json']));assert d['protocol_version']==1 and d['schema_version']==1
safe=[]
def op(name,args=[],cwd=repo):
 d=json.loads(run(ops[name]['argv']+args+['--repo-dir',str(cwd),'--json'],cwd));assert d['result'] in ['applied','no-op'],(name,d.get('result'),d.get('reason'))
 safe.append({'operation':name,'result':d['result']});(ev/'rehearsal-preparation.json').write_text(json.dumps(safe,indent=2)+'\n');return d
primary_template=json.loads(run(['python3',r/'check-results-template.py','--repo',repo]));assert primary_template['location']=='primary';(ev/'results-template-primary.json').write_text(json.dumps(primary_template,indent=2)+'\n')
arm=op('run.gate-before',['implement-next']);assert arm['armed'];private=r/'rehearsal-private.json';private.write_text(json.dumps(arm));private.chmod(0o600)
s=op('status');ch=s['changes'][0];assert ch['status']=='proposed';claim=op('change.claim',['--id','1','--version',ch['version'],'--gate-context',arm['dispatch_context']]);s=op('status');ch=s['changes'][0]
p=ev/'rehearsal-reconcile.json';p.write_text(json.dumps({'id':1,'version':ch['version'],'reconcile_log_entry':'Non-native setup compatibility rehearsal only. Synthetic plan below is never live certification evidence.'}));op('change.reconcile',['--input',str(p)])
s=op('status');ch=s['changes'][0];w=op('workspace.prepare',['--id','1','--version',ch['version']]);assert w['disposition']=='created';(ev/'workspace-created.json').write_text(json.dumps(w,indent=2)+'\n');f=Path(w['path'])
feature_template=json.loads(run(['python3',r/'check-results-template.py','--repo',f],f));assert feature_template['location']=='feature' and feature_template['sha256']==primary_template['sha256'];(ev/'results-template-feature.json').write_text(json.dumps(feature_template,indent=2)+'\n')
plan='docs/superpowers/plans/2026-09-14-rehearsal-greeting.md';p=f/plan;p.parent.mkdir(parents=True,exist_ok=True);p.write_text('# Rehearsal plan — not native evidence\n\n## Task 1: Trim greeting\n\nBuild profile: standard\n\nPreserve the three baseline cases, add six whitespace assertions, run baseline/RED/GREEN using configured go test -count=1 ./..., commit only greeting.go and greeting_test.go.\n')
op('artifact.backlink',['--artifact',plan,'--change',ch['path']],f);run(['git','add','--',plan],f);run(['git','commit','-m','Create synthetic rehearsal-only plan','-m','Docket-Plan-Path: '+plan],f);head=run(['git','rev-parse','HEAD'],f);s=op('status');op('change.attach-plan',['--id','1','--version',s['changes'][0]['version'],'--path',plan,'--commit',head])
print(run(['python3',r/'prepare-worker-inputs.py','--feature',f,'--plan-path',plan,'--routing-reason','One standard task; synthetic setup rehearsal only'],f))
print(json.dumps({'status':'rehearsal-preparation-passed','epoch_supplied':bool(arm.get('epoch')),'native_agents_started':False}))
