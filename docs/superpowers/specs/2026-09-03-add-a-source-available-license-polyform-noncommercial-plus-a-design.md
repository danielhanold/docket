<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0401 — Add a source-available license: PolyForm Noncommercial plus an individual commercial exemption](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0401-add-a-source-available-license-polyform-noncommercial-plus-a.md)**
<!-- docket:backlink:end -->

# Source-available license for docket — design

**Change:** 0401. **Type:** docs. **Path:** bounded (files added to an existing repo; no code flow).

## Goal

Give docket a stated license that (a) is free for personal use and for individuals working alone, (b) requires explicit written permission or a granted license for commercial use by any organization, and (c) covers the entire repository history, including every commit made before the license was added.

## License choice

**PolyForm Noncommercial 1.0.0**, verbatim, plus a separately authored additional-permissions notice.

Why this and not the alternatives:

- PolyForm Noncommercial is lawyer-drafted, permits any noncommercial purpose (with "Personal Uses" by individuals named explicitly, and noncommercial organizations such as charities, schools, public research, and government), and requires a separate license from the licensor for any commercial purpose. That is the owner's gate.
- Creative Commons BY-NC is not written for software and Creative Commons advises against using it for code.
- Commons Clause is a rider on Apache and is widely regarded as ambiguous.
- Business Source License 1.1 converts to open source on a change date, which is not wanted.
- Prosperity 3.0 is noncommercial but grants a 30-day commercial trial and is far less recognized.
- Elastic License 2.0 and SSPL permit commercial use and restrict only hosted offerings: the wrong shape.

PolyForm licenses must not be modified. Every extra term therefore lives in a **separate** file that the license file and README point to. The additional permissions can only *widen* what PolyForm grants, never narrow it, which is the intended direction (freelancers gain a grant PolyForm alone withholds).

Adding a license only grants rights: before it, every commit was all-rights-reserved by default. The owner authored every human commit; the remaining commits are AI-assisted output owned by the owner. There are no third-party contributors whose consent would be needed to license the history retroactively.

This is a **license, not terms of service**. A ToS governs a hosted service; docket is distributed source. The change and all files use the word "license". docket becomes *source-available*, not open source (PolyForm Noncommercial is not OSI-approved); the README says so plainly.

## Files

### `LICENSE` (repo root)

Contents, in order:

1. The PolyForm Noncommercial 1.0.0 **Required Notice** line, exactly in the form the license prescribes:

   ```
   Required Notice: Copyright Daniel Hanold (https://github.com/danielhanold/docket)
   ```

2. One pointer line (outside the license text):

   ```
   Additional permissions that widen this license are in LICENSE-ADDITIONAL-PERMISSIONS.md.
   ```

3. The full, unmodified text of PolyForm Noncommercial 1.0.0, fetched from https://polyformproject.org/licenses/noncommercial/1.0.0/ (plain-text form). The builder copies it byte-for-byte; no paraphrase, no reflow.

### `LICENSE-ADDITIONAL-PERMISSIONS.md` (repo root)

Exact text. The builder writes it as given; the only permitted edit is the contact address in clause 3 if the owner supplies a different one at build time.

```markdown
# Additional Permissions to the PolyForm Noncommercial License 1.0.0

These additional permissions are granted by the licensor, Daniel Hanold, on top of
the PolyForm Noncommercial License 1.0.0 in `LICENSE`. They only widen that license.
Where they and the license disagree, whichever grants you more permission applies.
Every other term of the license, including its conditions and its disclaimer of
warranty and liability, applies unchanged.

## 1. Individual commercial exemption

In addition to the noncommercial purposes the license permits, you may use the
software for any commercial purpose if you are an **individual working alone**:
a single natural person, whether acting in your own name or through a sole
proprietorship, single-member company, or similar vehicle that you alone own,
and that has no employees, contractors, or other workers besides you.

This exemption is personal to you. It does not extend to any organization with
more than one worker, and it ends if you begin using the software on behalf of,
or in work performed for, such an organization as its employee, contractor, or
agent, unless that organization holds its own license from the licensor.

Any other commercial use requires the licensor's prior written permission or a
separate license from the licensor.

## 2. Scope over the repository history

The license and these additional permissions apply to every version of the
software in this repository, including every commit, tag, and release made
before the license file was added. No earlier version is licensed on different
terms.

## 3. Obtaining a commercial license

To ask for written permission or a commercial license, contact Daniel Hanold at
danny@danielhanold.com. Written permission is only valid if it comes from the
licensor in writing; a public statement, an issue reply, or a chat message is
not a license unless it says so.
```

### `README.md`

Add a `## License` section near the end (after the existing final section; the builder places it last, before any trailing links block if one exists). Add a matching entry to the README's table of contents list. Exact text:

```markdown
## License

docket is **source-available, not open source**. It is licensed under the
[PolyForm Noncommercial License 1.0.0](LICENSE) with
[additional permissions](LICENSE-ADDITIONAL-PERMISSIONS.md). In short:

- **Personal and noncommercial use is free.** Individuals, charities, schools,
  public research, and government may use docket without asking.
- **Individuals working alone may use it commercially.** A freelancer or solo
  consultant with no employees or contractors is covered by the additional
  permissions.
- **Any other commercial use needs written permission.** Companies and other
  organizations must obtain explicit written permission or a separate license
  from the owner; see the additional-permissions file for how to ask.

The license applies to the whole history of this repository, including every
commit made before the license was added.
```

## Test

One new shell test, `tests/test_license_files.sh`, following the suite conventions in `tests/README.md`. It asserts:

- `LICENSE` and `LICENSE-ADDITIONAL-PERMISSIONS.md` exist at the repo root.
- `LICENSE` contains the exact identifier `PolyForm Noncommercial License 1.0.0` and the exact `Required Notice: Copyright Daniel Hanold` prefix.
- `LICENSE` contains the pointer to `LICENSE-ADDITIONAL-PERMISSIONS.md`.
- `LICENSE-ADDITIONAL-PERMISSIONS.md` contains the three headings `## 1. Individual commercial exemption`, `## 2. Scope over the repository history`, and `## 3. Obtaining a commercial license`.
- `README.md` contains a `## License` heading and links to both files.

Mutation-test it (per AGENTS.md): delete `LICENSE-ADDITIONAL-PERMISSIONS.md` or strip the `## License` heading and confirm the test reds. Grep patterns lead with fixed strings (`grep -qF -- "..."`) so no pattern is parsed as an option.

## Non-goals

- Terms of service or privacy policy for any hosted offering.
- Per-file license or SPDX headers.
- CLA or DCO; no external contributors exist.
- Commercial pricing or license terms beyond "how to ask".
- Any change to install or distribution mechanics.

## Caveat

The author of this spec is not a lawyer. The additional-permissions text is custom. The owner should have it reviewed before relying on it against a paying organization; that review is outside this change and does not block the build.
