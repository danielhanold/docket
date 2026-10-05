# v1.0.0-alpha.1 — public install check (Phase 5)

Run 2026-10-05 in a second, fresh test home (`$TEST_HOME_PUBLIC`): `HOME` and every `XDG_*` inside it, no `docket` on `PATH` beforehand (`command -v docket` → none), and no `DOCKET_RELEASE_BASE_URL`.

## Download

- `https://github.com/danielhanold/docket/releases/download/v1.0.0-alpha.1/install.sh`
- `https://github.com/danielhanold/docket/releases/download/v1.0.0-alpha.1/checksums.txt` — SHA-256 `2ab19e3d78a64895352561bbef3d3f35dadfca325d4052ae15f2f650ec153e27` (equal to the candidate's `checksums.txt`)

## Verify before running

```
$ grep '  install.sh$' checksums.txt | shasum -a 256 -c -
install.sh: OK
```

## Install

`sh install.sh --harness claude --bin-dir $TEST_HOME_PUBLIC/bin` (the downloaded script was run as a file; nothing was piped into `sh`). It finished with:

```
mode: release
harnesses: claude
asset set: sha256:933f1a317e1a542e4f56b22d762698a62a3b83013c0f04bf5c2ebf93c2d5e405 (protocol 1)
```

## Verify the install

```
docket version --json
{"protocol_version":1,"operation":"version","result":"applied","version":"v1.0.0-alpha.1","commit":"49e4af94b838ed11beb7ee32027eb122425632d9","build_date":"2026-10-04T22:17:38Z"}

docket install check --json
{"protocol_version":1,"operation":"install.check","result":"no-op","mode":"release","harnesses":["claude"],"asset_protocol":1,"asset_set_id":"sha256:933f1a317e1a542e4f56b22d762698a62a3b83013c0f04bf5c2ebf93c2d5e405","applied_work":false,"actions":[]}

docket diagnostic runtime --json
{"protocol_version":1,"operation":"diagnostic.runtime","result":"applied","go_version":"go1.26.8","go_os":"darwin","go_arch":"arm64","supported_target":true}
```

Result: pass. This also proves the download and checksum step of the upgrade guide (`docs/release/upgrading-from-bash.md`, section 3).
