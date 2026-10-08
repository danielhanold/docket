# v1.0.0-alpha.2 — public install check (Phase 5)

A second fresh test home (`$TEST_HOME`): no docket present, no `DOCKET_RELEASE_BASE_URL`, `HOME` and every `XDG_*` inside it.

## Downloads

- https://github.com/danielhanold/docket/releases/download/v1.0.0-alpha.2/install.sh — SHA-256 `f1fe6ddf66e0ed524d3a9d38222c88b140bef89e483d71f06d4970a70727917d`
- https://github.com/danielhanold/docket/releases/download/v1.0.0-alpha.2/checksums.txt — byte-equal to the accepted copy

## Verify before running

```
$ grep '  install.sh$' checksums.txt | shasum -a 256 -c -
install.sh: OK
```

## Install

```
$ sh install.sh --version v1.0.0-alpha.2 --harness cursor --bin-dir $TEST_HOME/bin
install: applied
mode: release
harnesses: cursor
asset set: sha256:d2437c7b1af99e29a452da850d3f479ff2e6aab9fd6f27b7d1e254133410d30d (protocol 1)
state: $TEST_HOME/.local/share/docket/state/install.json
collection:
  referenced  $TEST_HOME/.local/share/docket/versions/sha256-d2437c7b1af99e29a452da850d3f479ff2e6aab9fd6f27b7d1e254133410d30d  (retained by installed state or a live target)
actions:
install.check: no-op
mode: release
harnesses: cursor
asset set: sha256:d2437c7b1af99e29a452da850d3f479ff2e6aab9fd6f27b7d1e254133410d30d (protocol 1)
state: $TEST_HOME/.local/share/docket/state/install.json
```

## Checks

```
{"protocol_version":1,"operation":"version","result":"applied","version":"v1.0.0-alpha.2","commit":"ec4c2b1841954c2d6a3dd018e1133ee762861137","build_date":"2026-10-08T17:39:24Z"}
{"protocol_version":1,"operation":"install.check","result":"no-op","mode":"release","harnesses":["cursor"],"asset_protocol":1,"asset_set_id":"sha256:d2437c7b1af99e29a452da850d3f479ff2e6aab9fd6f27b7d1e254133410d30d","state_path":"$TEST_HOME/.local/share/docket/state/install.json","applied_work":false,"actions":[]}
{"protocol_version":1,"operation":"diagnostic.runtime","result":"applied","go_version":"go1.26.8","go_os":"darwin","go_arch":"arm64","supported_target":true}
```

Verdict: **pass** — `v1.0.0-alpha.2` at the candidate commit, `install check` clean (release mode, Cursor only), `supported_target: true`. This also proves the download and checksum step of the Bash upgrade guide.
