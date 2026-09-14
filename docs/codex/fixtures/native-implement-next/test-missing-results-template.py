import json,shutil,subprocess,tempfile
from pathlib import Path
package=Path(__file__).resolve().parent
with tempfile.TemporaryDirectory(prefix='docket-423-missing-template-') as td:
 root=Path(td).resolve();copy=root/'package';shutil.copytree(package,copy)
 (copy/'template/repo-snapshot/.agents/skills/docket-implement-next/results-template.md').unlink()
 run=subprocess.run(['python3',str(copy/'setup.py'),'--destination',str(root/'fixture')],capture_output=True,text=True)
 assert run.returncode!=0,'REGRESSION: setup accepted a fixture without its mandatory results template'
 assert 'required packaged results template missing' in run.stderr,run.stderr
 assert not (root/'fixture').exists(),'setup allocated a fixture before validating its mandatory source'
 print(json.dumps({'status':'passed','missing_results_template_rejected_by_setup':True}))
