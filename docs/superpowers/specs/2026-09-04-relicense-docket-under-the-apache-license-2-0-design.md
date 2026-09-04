<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0404 — Relicense docket under the Apache License 2.0](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0404-relicense-docket-under-the-apache-license-2-0.md)**
<!-- docket:backlink:end -->

# Apache License 2.0 for docket — design

**Change:** 0404. **Type:** docs. **Path:** bounded (files replaced, added, and removed at the repo root plus one guard test; no code flow).

## Goal

Move docket from PolyForm Noncommercial 1.0.0 plus a bespoke additional-permissions file (change 0401) to the Apache License 2.0, so that (a) any person or organization may use, modify, and redistribute docket without a bespoke grant, (b) attribution is preserved through Apache's `NOTICE` mechanism, (c) inbound contributions arrive under a standard, low-friction mechanism (the Developer Certificate of Origin), and (d) the license still covers the entire repository history.

## License choice

- **Apache License 2.0**, verbatim. OSI-approved, pre-cleared by most corporate open-source policies, and the conventional license for developer tooling. It carries an explicit patent grant (section 3) and a `NOTICE` mechanism (section 4(d)) that obliges redistributors to keep the attribution line.
- **Not MIT.** MIT has no patent grant and no `NOTICE` mechanism; attribution survives only as a copyright line inside the license file.
- **Not a source-available license** (PolyForm family, FSL, BSL). Each is non-OSI, each triggers a per-organization legal review, and the restriction each buys (no one may resell docket as a product) has no realistic counterparty for a solo-maintained CLI.
- **DCO, not CLA.** Apache's inbound-equals-outbound default plus a `Signed-off-by` trailer is the standard low-friction mechanism. A CLA is a barrier with no benefit unless the project intends to relicense contributed code later, which is not a goal.

Relicensing is a one-time act by the sole copyright holder. No external contribution has been merged, so no third-party consent is required.

## Files

### `LICENSE` (repo root)

The complete Apache License 2.0 text, byte-for-byte from the canonical source (`https://www.apache.org/licenses/LICENSE-2.0.txt`), including the `APPENDIX: How to apply the Apache License to your work` section. No preamble, no `Required Notice:` line, no pointer to any other file. The build step fetches the text and confirms a clean `diff` against the fetched file before committing.

### `NOTICE` (repo root, new)

```
docket
Copyright 2026 Daniel Hanold

This product is licensed under the Apache License, Version 2.0; see LICENSE.
```

Three lines of substance. Apache section 4(d) requires redistributors to carry these attribution notices forward.

### `LICENSE-ADDITIONAL-PERMISSIONS.md` (repo root)

Deleted. Every clause in it either widened the noncommercial license (moot under Apache) or described how to obtain a commercial license (no longer offered).

### `CONTRIBUTING.md` (repo root, new)

```markdown
# Contributing to docket

docket is licensed under the [Apache License 2.0](LICENSE). Contributions are
accepted under the same license.

## Developer Certificate of Origin

Every commit must carry a `Signed-off-by:` trailer certifying the
[Developer Certificate of Origin](https://developercertificate.org/): that you
wrote the change or otherwise have the right to submit it under the Apache
License 2.0. `git commit -s` adds the trailer. Pull requests whose commits lack
it are not merged.

## How work flows here

docket develops itself with its own workflow. Read `docs/changes/README.md` on
the `docket` branch and `tests/README.md` before starting: every change goes
through a change document, a build gate, and the full test suite, and the
rules in [CLAUDE.md](CLAUDE.md) apply to every edit. Bug reports and ideas are
welcome as issues; the maintainer captures them as changes deliberately.
```

### `README.md`

Replace the whole `## License` section (from its heading to the next `##` heading or end of file) with:

```markdown
## License

docket is open source under the [Apache License 2.0](LICENSE). The
[NOTICE](NOTICE) file carries the attribution that section 4(d) of the license
asks redistributors to preserve.

- **Files docket generates in your repository are yours.** Change documents,
  specs, plans, results, decision records, and configuration belong to you; the
  license places no condition on them.
- **The license grants no rights in the docket name.** Use the software freely;
  do not present a modified version as docket itself.
- **Contributions** are accepted under the Developer Certificate of Origin; see
  [CONTRIBUTING.md](CONTRIBUTING.md).

The license applies to the whole history of this repository, including every
commit made before it was added.
```

The retained last paragraph keeps 0401's whole-history statement; the licensor is the sole copyright holder, so it is a statement of fact rather than a grant that needs its own clause.

## Test

Retarget the existing guard rather than adding a second one.

`internal/repoguard/license_test.go`:

- `TestLicenseFiles` asserts, by fixed-string containment:
  - `LICENSE` contains `Apache License`, `Version 2.0, January 2004`, and the distinctive clause `"Licensed under the Apache License, Version 2.0 (the "License");`, which pins the appendix and therefore the complete text.
  - `LICENSE` does **not** contain `PolyForm` and does **not** contain `LICENSE-ADDITIONAL-PERMISSIONS.md`.
  - `NOTICE` contains `Copyright 2026 Daniel Hanold`.
  - `LICENSE-ADDITIONAL-PERMISSIONS.md` does not exist at the repo root (an `os.Stat` that must fail with not-exist; any other error is a test failure).
  - `CONTRIBUTING.md` contains `Signed-off-by`.
- `TestLicenseReadmeSection` keeps the `## License` heading assertion and asserts `](LICENSE)`, `](NOTICE)`, and `](CONTRIBUTING.md)` as markdown link destinations. Drop the `](LICENSE-ADDITIONAL-PERMISSIONS.md)` assertion.
- Update the doc comments on both tests so they describe the new artifacts and cite change 0404.

`tests/test_license_files.sh`: unchanged in mechanics. Its header comment lists the three files and the README section; rewrite that paragraph to name `LICENSE`, `NOTICE`, `CONTRIBUTING.md`, the absence assertion, and the README links. Both `--- PASS:` pins stay.

Mutation test at build time: rename `NOTICE` and watch `TestLicenseFiles` redden; restore. Reinsert the word `PolyForm` into `LICENSE` and watch it redden; restore.

Any edit to a Go file must be `gofmt`-clean before the suite runs; 0401's results record a review fix that was reverted for exactly that reason.

## Non-goals

- A contributor license agreement or CLA automation.
- SPDX or copyright headers on individual source files.
- Trademark registration or a standalone trademark policy.
- Behavioral changes to the binary, installer, or skills.
- A new ADR; the decision is recorded here and in the change's `## Why`, as 0401 did.

## Caveat

The author is not a lawyer. The Apache text is standard and unmodified, which is the point: the only custom wording this change introduces is the `NOTICE` line, the README sentences, and the DCO pointer.
