#!/usr/bin/env python3
"""Read-only primary snapshot comparison; cannot detect transient restored writes."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import stat
import subprocess

EXCLUDED = {'.git', '.docket', '.worktrees'}


def snapshot(repo):
    repo = Path(repo).resolve(strict=True)
    def git(*args):
        return subprocess.check_output(['git', '-C', str(repo), *args], text=True).strip()
    files = {}
    for base, dirs, names in os.walk(repo, followlinks=False):
        parent = Path(base)
        if parent == repo:
            dirs[:] = [x for x in dirs if x not in EXCLUDED]
            names = [x for x in names if x not in EXCLUDED]
        for name in list(dirs):
            if (parent / name).is_symlink():
                dirs.remove(name)
                names.append(name)
        for name in names:
            path = parent / name
            mode = path.lstat().st_mode
            value = {'mode': stat.S_IMODE(mode)}
            if stat.S_ISLNK(mode):
                value.update(type='symlink', target=os.readlink(path))
            elif stat.S_ISREG(mode):
                value.update(type='file', sha256=hashlib.sha256(path.read_bytes()).hexdigest())
            else:
                value.update(type='special')
            files[str(path.relative_to(repo))] = value
    return {'repo': str(repo), 'head': git('rev-parse', 'HEAD'), 'branch': git('symbolic-ref', 'HEAD'), 'index': git('ls-files', '--stage'), 'files': files, 'excluded_administration': sorted(EXCLUDED)}


if __name__ == '__main__':
    p = argparse.ArgumentParser()
    p.add_argument('mode', choices=['snapshot', 'check'])
    p.add_argument('--repo', required=True)
    p.add_argument('--baseline')
    args = p.parse_args()
    current = snapshot(args.repo)
    if args.mode == 'snapshot':
        print(json.dumps(current, indent=2))
    else:
        if not args.baseline:
            p.error('--baseline is required for check')
        before = json.loads(Path(args.baseline).read_text())
        changed = [x for x in sorted(set(before['files']) | set(current['files'])) if before['files'].get(x) != current['files'].get(x)]
        other = [x for x in ('repo', 'head', 'branch', 'index', 'excluded_administration') if before.get(x) != current.get(x)]
        print(json.dumps({'status': 'PRIMARY_UNCHANGED' if not changed and not other else 'PRIMARY_CHANGED', 'changed_paths': changed, 'changed_state': other, 'limitation': 'tool evidence must also exclude temporary writes'}, indent=2))
        raise SystemExit(bool(changed or other))
