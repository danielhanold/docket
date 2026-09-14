#!/usr/bin/env python3
"""Create one fresh disposable POC; no agents, claim, worktree, scope or gate."""
import argparse,datetime,hashlib,importlib.util,json,os,shutil,stat,subprocess,sys,tomllib
from pathlib import Path
PACKAGE=Path(__file__).resolve().parent
sys.dont_write_bytecode=True

def run(argv,cwd=None):
 p=subprocess.run([str(a) for a in argv],cwd=cwd,text=True,capture_output=True)
 if p.returncode:raise RuntimeError(p.stdout+'\n'+p.stderr)
 return p.stdout.strip()
def write(p,value):
 p.parent.mkdir(parents=True,exist_ok=True);p.write_text(value if isinstance(value,str) else json.dumps(value,indent=2)+'\n')
def main():
 parser=argparse.ArgumentParser();parser.add_argument('--destination',required=True,type=Path);a=parser.parse_args()
 root=a.destination.absolute();assert not root.exists(),'destination must not exist';assert root.parent.resolve()==root.parent,'canonical parent required'
 c=json.loads(run(['docket','capabilities','--json']))
 assert c['protocol_version']==1 and c['capability_version']==1 and c['operation']=='capabilities' and c['result']=='applied'
 assert c['commands'] and len({x['id'] for x in c['commands']})==len(c['commands'])
 for x in c['commands']:assert x['argv'] and set(x['effects'])<={'read','local-write','metadata-write','external-write','process-control'}
 ops={x['id']:x for x in c['commands']};schemas={}
 for name in ['change.create','change.groom']:
  d=json.loads(run(ops['schema']['argv']+['--operation',name,'--json']));assert d['protocol_version']==1 and d['schema_version']==1 and d['result']=='applied';schemas[name]=next(x for x in d['operations'] if x['id']==name)['request']['fields']
 resource_spec=importlib.util.spec_from_file_location('results_template',PACKAGE/'template/check-results-template.py');resource=importlib.util.module_from_spec(resource_spec);resource_spec.loader.exec_module(resource)
 template_source=PACKAGE/'template/repo-snapshot'/resource.RELATIVE
 assert template_source.is_file() and '## Outcome\n' in template_source.read_text(),'required packaged results template missing'
 root.mkdir();repo=root/'repo';ev=root/'evidence';ev.mkdir();origin=root/'origin.git'
 shutil.copytree(PACKAGE/'template/repo-snapshot',repo)
 for p in (PACKAGE/'template').iterdir():
  if p.is_file():shutil.copy2(p,root/p.name)
 for p in root.rglob('*'):
  if p.is_file():p.write_text(p.read_text().replace('@@FIXTURE_ROOT@@',str(root)))
 for name in ['check-boundary.py','check-task-inputs.py','check-runtime.py','primary-audit.py','validate-plan-payload.py','prepare-worker-inputs.py','check-results-template.py']:os.chmod(root/name,0o755)
 for source,target in [('POLICY.md','AGENTS.md'),('RUN.md','RUN.md'),('CODEX-WORKTREE.md','.codex/POC-WORKTREE.md'),('WORKER.md','.codex/POC-WORKER.md'),('WORKER-DISPATCH.md','.codex/POC-WORKER-DISPATCH.md'),('PLAN-DISPATCH.md','.codex/POC-PLAN-DISPATCH.md')]:write(repo/target,(root/source).read_text())
 bindings=json.loads((repo/'.codex/poc-skill-bindings.json').read_text())
 for name,x in bindings.items():x['sha256']=hashlib.sha256((repo/x['path']).read_bytes()).hexdigest();x['source']='package pinned snapshot; see PROVENANCE.json'
 write(repo/'.codex/poc-skill-bindings.json',bindings)
 run(['git','init','-b','main',repo]);run(['git','config','user.name','Docket POC'],repo);run(['git','config','user.email','docket-poc@localhost'],repo)
 run(['git','add','-f','--','.'],repo);run(['git','commit','-m','Initialize isolated native continuous POC'],repo)
 run(['git','switch','--orphan','docket'],repo);run(['git','commit','--allow-empty','-m','Initialize isolated metadata branch'],repo);run(['git','switch','main'],repo)
 run(['git','init','--bare',origin]);run(['git','--git-dir',origin,'symbolic-ref','HEAD','refs/heads/main']);run(['git','remote','add','origin',origin],repo);run(['git','push','-u','origin','main','docket'],repo);run(['git','remote','set-head','origin','main'],repo)
 receipts=[]
 def op(name,extra=[]):
  d=json.loads(run(ops[name]['argv']+extra+['--repo-dir',str(repo),'--json'],repo));assert d['protocol_version']==1 and d['operation']==name and d['result'] in ['applied','no-op'],d
  receipts.append(d);write(ev/'preparation-operations.json',receipts);return d
 def request(name,body):
  fs=schemas[name];assert set(body)<={x['key'] for x in fs};assert all(x['key'] in body for x in fs if x.get('required'))
  path=ev/(name+'-request.json');write(path,body);return op(name,['--request',str(path)])
 op('repository.prepare');config=op('diagnostic.config');assert config['mutation_allowed'];assert config['effective']['build']['test_command']['value']=='go test -count=1 ./...'
 made=request('change.create',json.loads((PACKAGE/'candidate.json').read_text()));assert made['id']==1
 s=op('status');ch=s['changes'][0]
 request('change.groom',{'change_id':1,'path':ch['path'],'version':ch['version'],'outcome':'spec','spec_markdown':(PACKAGE/'candidate-spec.md').read_text()+'\n## Native continuous POC boundary\n\n'+(root/'POLICY.md').read_text(),'depends_on':[],'related':[],'discovered_from':[],'adrs':[]})
 op('repository.prepare')
 s=op('status');assert s['ready']==[1] and len(s['changes'])==1 and s['changes'][0]['status']=='proposed'
 assert not (repo/'.worktrees').exists();assert not run(['git','ls-files','docs/superpowers/plans','docs/results'],repo);assert not run(['git','status','--porcelain'],repo)
 baseline=run(['go','test','-count=1','./...'],repo)
 spec=importlib.util.spec_from_file_location('audit',root/'primary-audit.py');audit=importlib.util.module_from_spec(spec);spec.loader.exec_module(audit);write(ev/'primary-baseline.json',audit.snapshot(repo))
 roles={p.stem:{k:v for k,v in tomllib.loads(p.read_text()).items() if k in ['model','model_reasoning_effort']} for p in (repo/'.codex/agents').glob('*.toml')}
 info={'status':'prepared-not-executed','repo':str(repo),'origin':str(origin),'candidate':1,'binary':c['binary'],'primary_head':run(['git','rev-parse','HEAD'],repo),'roles':roles,'gate_armed':False,'candidate_claimed':False,'workspace_allocated':False,'plan_present':False,'live_agents_started':False,'baseline_tests':baseline,'prepared_at':datetime.datetime.now(datetime.timezone.utc).isoformat()};write(root/'arm.json',info)
 immutable=[p for p in root.iterdir() if p.is_file()]+[repo/p for p in run(['git','ls-files'],repo).splitlines()]+[ev/'primary-baseline.json']
 write(ev/'launch-manifest.json',{'binary':c['binary'],'files':{str(p.relative_to(root)):{'sha256':hashlib.sha256(p.read_bytes()).hexdigest(),'mode':f'{stat.S_IMODE(p.stat().st_mode):04o}'} for p in immutable}})
 write(ev/'preparation-results-template.json',resource.validate(repo,root))
 print(json.dumps(info,indent=2))
if __name__=='__main__':main()
