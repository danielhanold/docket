#!/usr/bin/env python3
"""Read-only POC payload validation; no dispatch, Docket mutation, or plan authoring."""
import argparse
import hashlib
import json
from pathlib import Path

FIELDS = set('change_id title primary_checkout feature_worktree feature_ref pre_dispatch_head workspace_receipt boundary_checker metadata_worktree change_file spec_file backlink_change_path plan_skill plan_skill_file build_skill build_skill_file learnings_enabled learnings_index test_command build_profile task_count preserve_baseline_tests assignment'.split())


def require(ok, why):
    if not ok:
        raise ValueError(why)


def path(value, directory=False):
    require(isinstance(value, str) and bool(value), 'missing path')
    p = Path(value)
    require(p.is_absolute() and str(p.resolve(strict=True)) == value, 'path is not canonical/absolute: ' + value)
    require(p.is_dir() if directory else p.is_file(), 'path type mismatch: ' + value)
    return p


def validate(data):
    require(isinstance(data, dict), 'payload must be an object')
    missing = FIELDS - data.keys()
    require(not missing, 'missing fields: ' + ', '.join(sorted(missing)))
    require(not data.keys() - FIELDS, 'unexpected payload fields')
    for key in FIELDS - {'change_id', 'task_count', 'learnings_enabled', 'learnings_index', 'preserve_baseline_tests'}:
        require(isinstance(data[key], str) and data[key].strip(), 'empty or non-string field: ' + key)
    require(type(data['change_id']) is int and data['change_id'] == 1, 'wrong change id')
    require(type(data['task_count']) is int and data['task_count'] == 1, 'one task required')
    require(data['learnings_enabled'] is False and data['learnings_index'] is None, 'learnings must be explicitly disabled')
    require(data['preserve_baseline_tests'] is True and data['build_profile'] == 'standard', 'wrong task constraints')
    require(data['plan_skill'] == 'superpowers:writing-plans' and data['build_skill'] == 'docket-build', 'wrong skill binding')
    require(data['test_command'] == 'go test -count=1 ./...', 'wrong fixture test command')
    require(data['feature_ref'].startswith('refs/heads/') and len(data['pre_dispatch_head']) == 40, 'invalid ref/head')
    primary, feature, metadata = [path(data[k], directory=True) for k in ['primary_checkout', 'feature_worktree', 'metadata_worktree']]
    require(feature != primary, 'feature cannot be primary')
    change, spec = [path(data[k]) for k in ['change_file', 'spec_file']]
    require(change.is_relative_to(metadata) and spec.is_relative_to(metadata), 'change/spec must be in synchronized metadata')
    relative = Path(data['backlink_change_path'])
    require(not relative.is_absolute() and '..' not in relative.parts and metadata / relative == change, 'wrong backlink home')
    plan_skill, build_skill = [path(data[k]) for k in ['plan_skill_file', 'build_skill_file']]
    require(plan_skill == feature / '.agents/skills/superpowers-writing-plans/SKILL.md', 'wrong plan skill snapshot path')
    require(build_skill == feature / '.agents/skills/docket-build/SKILL.md', 'wrong build skill snapshot path')
    manifest = json.loads((feature / '.codex/poc-skill-bindings.json').read_text())
    for key, file in [('superpowers:writing-plans', plan_skill), ('docket-build', build_skill)]:
        require(hashlib.sha256(file.read_bytes()).hexdigest() == manifest[key]['sha256'], 'skill snapshot changed: ' + key)
    path(data['workspace_receipt']); path(data['boundary_checker'])
    return {'status': 'PLANNER_PAYLOAD_OK', 'field_count': len(FIELDS), 'plan_skill': data['plan_skill'], 'plan_skill_file': str(plan_skill), 'change_file': str(change), 'spec_file': str(spec), 'limitations': 'coordinator separately verifies live claim, boundary, branch/HEAD and exact dispatch message'}


if __name__ == '__main__':
    p = argparse.ArgumentParser()
    p.add_argument('--payload', required=True)
    args = p.parse_args()
    try:
        print(json.dumps(validate(json.loads(Path(args.payload).read_text())), indent=2))
    except (ValueError, OSError, KeyError) as e:
        print(json.dumps({'status': 'PLANNER_PAYLOAD_FAIL', 'reason': str(e)}))
        raise SystemExit(1)
