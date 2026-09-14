#!/usr/bin/env python3
"""Read-only direct lookup of the coordinator's required results template."""
import argparse,hashlib,json,subprocess,sys
from pathlib import Path
ROOT=Path(__file__).resolve().parent
RELATIVE=Path('.agents/skills/docket-implement-next/results-template.md')

def require(ok,reason):
 if not ok:raise ValueError('RESULTS_TEMPLATE_FAIL: '+reason)
def validate(repo,root=ROOT):
 require(repo.is_absolute() and repo.resolve()==repo,'noncanonical repository path')
 primary=root/'repo'
 require(repo==primary or repo.parent==primary/'.worktrees','repository outside fixture')
 def git(*args):return subprocess.check_output(['git','-C',str(repo),*args],text=True).strip()
 require(Path(git('rev-parse','--show-toplevel'))==repo,'repository root mismatch')
 registered=[line[len('worktree '):] for line in git('worktree','list','--porcelain').splitlines() if line.startswith('worktree ')]
 require(str(repo) in registered,'unregistered worktree')
 require(Path(git('rev-parse','--path-format=absolute','--git-common-dir')).resolve()==primary/'.git','foreign repository')
 path=repo/RELATIVE
 require(path.resolve()==path and path.is_file(),'required template missing or redirected: '+str(path))
 require(git('ls-files','--',str(RELATIVE))==str(RELATIVE),'template not tracked')
 raw=path.read_bytes();sha=hashlib.sha256(raw).hexdigest()
 expected=json.loads((root/'evidence/launch-manifest.json').read_text())['files']['repo/'+str(RELATIVE)]['sha256']
 require(sha==expected,'template differs from launch manifest')
 committed=subprocess.check_output(['git','-C',str(repo),'show','HEAD:'+str(RELATIVE)])
 require(hashlib.sha256(committed).hexdigest()==expected,'template differs from committed source')
 require('## Outcome\n' in raw.decode(),'required Outcome section missing')
 return {'status':'RESULTS_TEMPLATE_OK','repo':str(repo),'location':'primary' if repo==primary else 'feature','results_template_file':str(path),'sha256':sha,'source_head':git('rev-parse','HEAD')}
def main():
 p=argparse.ArgumentParser();p.add_argument('--repo',required=True,type=Path);a=p.parse_args();print(json.dumps(validate(a.repo),indent=2))
if __name__=='__main__':
 try:main()
 except (ValueError,KeyError,OSError,subprocess.CalledProcessError) as e:print(json.dumps({'status':'RESULTS_TEMPLATE_FAIL','reason':str(e)}));sys.exit(1)
