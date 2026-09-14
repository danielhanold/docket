#!/usr/bin/env python3
"""Read-only fixed-record/entry check; does not execute tasks or change cwd."""
import argparse,hashlib,importlib.util,json,sys
from pathlib import Path
sys.dont_write_bytecode=True
ROOT=Path(__file__).resolve().parent

def validate(data,root):
    required={'change_id','task_id','phase','primary_checkout','feature_worktree','branch','feature_ref','pre_dispatch_head','workspace_receipt','boundary_checker','operation_reference','plan_path','plan_file','build_profile','routing_reason','worker_run_root','test_command','test_argv','skill_files','assignment','input_version'}
    if set(data)!=required:raise ValueError('fixed task input keys missing or unexpected; credentials are not static inputs')
    primary=root/'repo';feature=Path(data['feature_worktree'])
    if data['input_version']!=1 or data['change_id']!=1 or data['task_id']!='task-1' or data['phase']!='build' or data['build_profile']!='standard':raise ValueError('task identity mismatch')
    if data['primary_checkout']!=str(primary) or feature.parent!=primary/'.worktrees':raise ValueError('wrong primary or feature root')
    if feature.resolve()!=feature or primary.resolve()!=primary:raise ValueError('noncanonical repository path')
    if data['worker_run_root']!=str(root/'evidence/worker-gates') or Path(data['worker_run_root']).resolve()!=Path(data['worker_run_root']):raise ValueError('wrong task run-root')
    if data['workspace_receipt']!=str(root/'evidence/workspace-created.json') or data['boundary_checker']!=str(root/'check-boundary.py'):raise ValueError('wrong entry input')
    if data['feature_ref']!='refs/heads/'+data['branch']:raise ValueError('branch/ref mismatch')
    if data['test_argv']!=['go','test','-count=1','./...'] or data['test_command']!='go test -count=1 ./...':raise ValueError('unexpected configured test')
    if data['plan_file']!=str(feature/data['plan_path']) or data['operation_reference']!=str(feature/'.codex/POC-WORKTREE.md'):raise ValueError('wrong feature artifact path')
    if set(data['skill_files'])!={'docket-build-task','docket-convention','docket-build','gate-caller-loop'}:raise ValueError('missing skill input')
    for value in [data['plan_file'],data['operation_reference'],*data['skill_files'].values()]:
        p=Path(value)
        if not p.is_absolute() or not p.resolve().is_relative_to(feature) or p.resolve()!=p or not p.is_file():raise ValueError('missing or escaping feature input')
    return data

def load_checked(path,digest):
    if path!=ROOT/'worker-inputs.json' or path.resolve()!=path:raise ValueError('wrong fixed input file')
    raw=path.read_bytes()
    if hashlib.sha256(raw).hexdigest()!=digest:raise ValueError('fixed task input hash mismatch')
    return validate(json.loads(raw),ROOT)

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--inputs',required=True,type=Path);parser.add_argument('--sha256',required=True);args=parser.parse_args()
    data=load_checked(args.inputs,args.sha256)
    if str(Path.cwd().resolve())!=data['primary_checkout']:raise ValueError('native startup cwd is not the fixture primary')
    spec=importlib.util.spec_from_file_location('boundary',ROOT/'check-boundary.py');module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
    receipt=module.check(data['primary_checkout'],data['feature_worktree'],data['feature_ref'],data['pre_dispatch_head'],data['workspace_receipt'],data['change_id'])
    receipt.update(task_input_file=str(args.inputs),task_input_sha256=args.sha256,worker_run_root=data['worker_run_root'])
    print(json.dumps(receipt,indent=2))

if __name__=='__main__':
    try:main()
    except (ValueError,OSError,KeyError,TypeError) as e:
        print(json.dumps({'status':'BINDING_FAIL','reason':str(e),'startup_cwd':str(Path.cwd())}));sys.exit(1)
