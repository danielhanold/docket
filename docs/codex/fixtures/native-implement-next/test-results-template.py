"""Exercise hidden tracked template lookup in primary and a real linked worktree."""
import hashlib,importlib.util,json,subprocess,sys,tempfile
from pathlib import Path
sys.dont_write_bytecode=True
package=Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('template_check',package/'template/check-results-template.py');m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
checks=[]
with tempfile.TemporaryDirectory(prefix='docket-423-template-check-') as td:
 root=Path(td).resolve();repo=root/'repo';repo.mkdir();(root/'evidence').mkdir()
 def git(*args):return subprocess.check_output(['git','-C',str(repo),*args],stderr=subprocess.DEVNULL,text=True).strip()
 git('init','-b','main');git('config','user.name','Fixture test');git('config','user.email','test@localhost')
 path=repo/m.RELATIVE;path.parent.mkdir(parents=True);raw=(package/'template/repo-snapshot'/m.RELATIVE).read_bytes();path.write_bytes(raw)
 git('add','-f','.');git('commit','-m','Fixture template')
 sha=hashlib.sha256(raw).hexdigest();manifest=root/'evidence/launch-manifest.json';manifest.write_text(json.dumps({'files':{'repo/'+str(m.RELATIVE):{'sha256':sha}}}))
 hidden=subprocess.run(['rg','--files',str(repo)],capture_output=True,text=True);assert str(path) not in hidden.stdout.splitlines();checks.append('original default search omits existing hidden template')
 assert m.validate(repo,root)['results_template_file']==str(path);checks.append('direct primary lookup passes despite hidden directory')
 feature=repo/'.worktrees/feature';git('worktree','add','-b','feature',str(feature))
 proof=m.validate(feature,root);assert proof['results_template_file']==str(feature/m.RELATIVE) and proof['location']=='feature';checks.append('registered feature lookup returns feature path and correct hash')
 def reject(label,fn):
  try:fn()
  except (ValueError,KeyError,subprocess.CalledProcessError):checks.append(label);return
  raise AssertionError('guard accepted '+label)
 f=feature/m.RELATIVE;f.unlink();reject('missing feature template rejected',lambda:m.validate(feature,root));f.write_bytes(raw)
 f.write_bytes(raw+b'changed\n');reject('modified feature template rejected',lambda:m.validate(feature,root));f.write_bytes(raw)
 f.unlink();f.symlink_to(path);reject('feature symlink to primary rejected',lambda:m.validate(feature,root));f.unlink();f.write_bytes(raw)
 reject('noncanonical path rejected',lambda:m.validate(feature/'..',root))
 reject('outside fixture rejected',lambda:m.validate(root/'other',root))
 original=manifest.read_bytes();manifest.write_text(json.dumps({'files':{'repo/'+str(m.RELATIVE):{'sha256':'0'*64}}}));reject('manifest digest mismatch rejected',lambda:m.validate(feature,root));manifest.write_bytes(original)
 assert m.validate(feature,root)['sha256']==sha;assert git('status','--porcelain')=='?? .worktrees/' or git('status','--porcelain')==''
 checks.append('restored template passes after mutations')
print(json.dumps({'status':'passed','checks':checks,'check_count':len(checks),'scope':'read-only artifact lookup and mutation checks; no agents or Docket gates'},indent=2))
