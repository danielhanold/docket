#!/usr/bin/env python3
"""Read-only entry check, not a launcher or filesystem sandbox."""
import argparse
import json
import os
from pathlib import Path
import subprocess


def require(condition, message):
    if not condition:
        raise ValueError(message)


def git(root, *args):
    return subprocess.check_output(['git', '-C', str(root), *args], text=True, stderr=subprocess.PIPE).strip()


def canonical(value):
    p = Path(value)
    require(p.is_absolute(), 'path must be absolute')
    q = p.resolve(strict=True)
    require(str(q) == value, 'path must be the exact canonical root')
    require(q.is_dir(), 'path must be a directory')
    return q


def check(primary, feature, ref, head, receipt, change_id):
    for key in ('GIT_DIR', 'GIT_WORK_TREE', 'GIT_INDEX_FILE', 'GIT_COMMON_DIR'):
        require(key not in os.environ, 'Git redirect environment is not permitted: ' + key)
    primary, feature = canonical(primary), canonical(feature)
    require(feature != primary and feature not in primary.parents, 'feature must not contain primary')
    require(git(feature, 'rev-parse', '--show-toplevel') == str(feature), 'feature is not a worktree root')
    require(git(primary, 'rev-parse', '--show-toplevel') == str(primary), 'primary is not a repository root')
    common = git(primary, 'rev-parse', '--path-format=absolute', '--git-common-dir')
    require(git(feature, 'rev-parse', '--path-format=absolute', '--git-common-dir') == common, 'wrong repository')
    require(git(feature, 'rev-parse', '--absolute-git-dir') != common, 'feature is not a linked worktree')
    require(ref.startswith('refs/heads/') and git(feature, 'symbolic-ref', 'HEAD') == ref, 'wrong feature ref')
    require(len(head) == 40 and git(feature, 'rev-parse', 'HEAD') == head, 'moved or invalid HEAD')
    require(not git(feature, 'status', '--porcelain', '--untracked-files=all'), 'feature is dirty')
    records = git(primary, 'worktree', 'list', '--porcelain').split('\n\n')
    require(any(('worktree ' + str(feature)) in r.splitlines() and ('branch ' + ref) in r.splitlines() for r in records), 'unregistered feature worktree')
    data = json.loads(Path(receipt).read_text())
    require(data.get('protocol_version') == 1 and data.get('operation') == 'workspace.prepare', 'invalid workspace receipt')
    require(data.get('result') == 'applied' and data.get('disposition') == 'created', 'workspace was not created during this run')
    require(data.get('id') == change_id and data.get('path') == str(feature) and data.get('feature_ref') == ref, 'workspace ownership receipt mismatch')
    return {'status': 'BINDING_OK', 'startup_cwd': str(Path.cwd().resolve()), 'primary': str(primary), 'feature': str(feature), 'feature_ref': ref, 'pre_dispatch_head': head, 'change_id': change_id, 'limitation': 'entry check only; live Docket claim and subsequent tool targeting require separate verification'}


if __name__ == '__main__':
    p = argparse.ArgumentParser()
    for key in ('primary', 'feature', 'ref', 'head', 'receipt'):
        p.add_argument('--' + key, required=True)
    p.add_argument('--change-id', type=int, required=True)
    try:
        print(json.dumps(check(**vars(p.parse_args())), indent=2))
    except (ValueError, OSError, subprocess.CalledProcessError) as e:
        print(json.dumps({'status': 'BINDING_FAIL', 'reason': str(e), 'startup_cwd': str(Path.cwd())}))
        raise SystemExit(1)
