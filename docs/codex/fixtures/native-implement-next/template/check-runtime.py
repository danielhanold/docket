#!/usr/bin/env python3
"""Read only this current task's runtime records; never launch or reconfigure it."""
import json,os,re,sys
from pathlib import Path
ROOT=Path(__file__).resolve().parent

def validate(records,thread_id,primary):
    metas=[r['payload'] for r in records if r.get('type')=='session_meta']
    if not metas or any(m.get('id')!=thread_id for m in metas): raise ValueError('runtime task identity missing or mismatched')
    if any(m.get('cwd')!=primary for m in metas): raise ValueError('host startup cwd is not the fixture primary')
    if any(isinstance(m.get('source'),dict) and 'subagent' in m['source'] for m in metas): raise ValueError('controller must be a root task')
    contexts=[r for r in records if r.get('type')=='turn_context']
    if not contexts: raise ValueError('runtime model context unavailable')
    current=max(contexts,key=lambda r:r.get('timestamp',''))['payload']
    if current.get('cwd')!=primary: raise ValueError('current runtime cwd is not the fixture primary')
    if (current.get('model'),current.get('effort'))!=('gpt-5.6-terra','low'): raise ValueError('current controller runtime must be gpt-5.6-terra / low')
    return {'status':'RUNTIME_OK','thread_id':thread_id,'model':current['model'],'effort':current['effort'],'startup_cwd':primary,'source':metas[-1].get('source'),'cli_version':metas[-1].get('cli_version')}

def main():
    primary=str(ROOT/'repo')
    if str(Path.cwd().resolve())!=primary: raise ValueError('run this check from the fixture primary checkout')
    tid=os.environ.get('CODEX_THREAD_ID','')
    if not re.fullmatch(r'[0-9a-fA-F-]{36}',tid): raise ValueError('current CODEX_THREAD_ID unavailable')
    home=Path(os.environ.get('CODEX_HOME',str(Path.home()/'.codex')))
    paths=sorted((home/'sessions').glob('**/*'+tid+'*.jsonl'))
    if not paths: raise ValueError('targeted current-task host log unavailable')
    records=[];used=[]
    for p in paths:
        selected=[]
        with p.open() as stream:
            for line in stream:
                try:r=json.loads(line)
                except ValueError: continue
                if r.get('type') in ['session_meta','turn_context']:selected.append(r)
        if not selected or selected[0].get('type')!='session_meta' or selected[0]['payload'].get('id')!=tid:continue
        records.extend(selected);used.append(str(p))
    result=validate(records,tid,primary);result['source_files']=used
    print(json.dumps(result,indent=2))

if __name__=='__main__':
    try:main()
    except (ValueError,OSError,KeyError) as e:
        print(json.dumps({'status':'RUNTIME_FAIL','reason':str(e)}));sys.exit(1)
