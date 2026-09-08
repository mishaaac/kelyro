# Multi-source verification v2

`multi-source-verification-v2` preserves every acceptance, corroboration,
scope, conflict, and confidence rule from
`multi-source-verification-v1`. It changes one closed handoff exposed by live
dogfooding: a Trust Decision whose state is `requires_verification` is no
longer categorically unable to participate in verification.

## Provisional authoritative support

V2 may produce `verified_with_caveat` when all of these conditions hold:

- the v1 outcome would otherwise be `insufficient_evidence`;
- the Trust Decision is `requires_verification` and Tier A or Tier B;
- the Source kind is specification, standard, official documentation, release
  notes, official blog, package reference, official tutorial, or source code;
- no matched Registry entry is blocked or deprecated;
- every Source is temporally and version-scope consistent with the Claim;
- the literal Claim statement is topically anchored to its stored topic; and
- no assessed Source is explicitly rejected.

For normative Claims, the pending Source must additionally be a v1 primary
kind. Production and general Claims may use any authoritative kind in the list
above. Security and community Claims are never promoted by this rule and keep
their v1 authority/corroboration requirements.

The new reason code is
`authoritative_source_requires_verification`. This route can never produce
`verified`; its maximum status and confidence cap are
`verified_with_caveat` and `0.75`. Missing reviewed organization ownership
remains visible through `organization_unknown` and zero independent accepted
organizations.

## Compatibility and persistence

The pure `Verify` entry point remains v1 for historical callers and fixtures;
the production application service invokes `VerifyV2`. Bundle assembly accepts
both immutable verification versions and applies the same status handling.

Forward-only SQLite migration v47 adds `verification_results_v2` instead of
rewriting the published v34 table or its checks. The adapter writes v2 rows to
the new table and reads latest history across both tables. Existing legacy and
v1 rows remain unchanged and readable.

The policy performs no network access, does not seed the Source Registry, does
not convert the underlying Trust Decision to accepted, and does not alter
Student Core or curriculum state.
