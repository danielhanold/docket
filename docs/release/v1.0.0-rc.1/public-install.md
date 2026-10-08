# v1.0.0-rc.1 — public install check (Phase 5)

Fresh test home (`$TEST_HOME`, no Docket present, no `DOCKET_RELEASE_BASE_URL`, `env -i` with `HOME` and every `XDG_*` inside it).

1. Downloaded `install.sh` and `checksums.txt` from `https://github.com/danielhanold/docket/releases/download/v1.0.0-rc.1/`.
2. Verified before running: `grep '  install.sh$' checksums.txt | shasum -a 256 -c -` → `install.sh: OK` (`52fb4dcb78648db02948632713a08120443abe15681b19e338e139bfb6321608`).
3. `sh install.sh --version v1.0.0-rc.1 --harness claude --harness cursor --bin-dir $TEST_HOME/bin` installed `docket` and `dckt`.

| Probe | Result |
|---|---|
| `docket version --json` | `v1.0.0-rc.1`, commit `d0f4712a53bd6a158fddada91cdf612f1755a531` |
| `docket install check --json` | `no-op`, `mode: release`, harnesses `[claude, cursor]`, no actions |
| `docket diagnostic runtime --json` | `go1.26.8`, darwin/arm64, `supported_target: true` |
| `xattr $TEST_HOME/bin/docket` | `com.apple.provenance` only; no `com.apple.quarantine` |
| `codesign -v $TEST_HOME/bin/docket` | valid; `Signature=adhoc`, `linker-signed` |

The curl-and-tar install path carries no quarantine attribute, so Gatekeeper never assesses the binary.
