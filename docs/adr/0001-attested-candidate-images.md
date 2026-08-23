# Candidate images get their SHA tag only after attestation

CI builds each candidate image to a run-scoped staging tag, resolves its exact
digest, attests that digest, verifies the attestation, and only then adds the
`:<commit-sha>` tag and deletes the staging tag. The obvious
alternative — build and push straight to the SHA tag, then attest — is roughly
twenty lines shorter, and it is what a reader will be tempted to collapse this
back into.

## Why

Two reasons, and the second is the one that is easy to miss.

**A compromised registry credential must not be able to supply a release
input.** `GCP_CANDIDATE_SA_KEY` is a long-lived JSON service account key with
write access to the dev registry, and this repository is public. Without
attestation, anyone holding that key could push an image under a legitimate
commit SHA tag and the resolver would deploy it to prod. Neither registry
enables immutable tags — see "Tag mutability" in `docs/operations.md` for why
that is deliberate — so attestation is the only control that rejects a tag
pointing at a digest CI never built. It is load-bearing on its own.

**A SHA tag published before attestation cannot be repaired by CI.** If CI
pushes the SHA tag and then fails before attesting, that digest can never be
attested afterwards: the release path refuses to re-attest an existing digest,
by design. Because tags here are mutable, the commit is not permanently lost —
but removing the tag needs `artifactregistry.tags.delete`, which the candidate
identity does not hold. Recovery therefore stops being automatic and becomes a
registry-admin break-glass action, or a quarantined SHA and a fresh
release-impacting commit. The staging tag exists so that an interrupted run
leaves a throwaway tag that nothing consults, instead of a published one that
requires a privileged human to clean up.

## Consequences

Enabling `--immutable-tags` later would strengthen the first reason and turn the
second into a genuinely unrecoverable commit rather than an awkward one. It
would also block deletion of tagged images and break the cleanup policies both
repositories depend on, so it is not a free hardening step.

Moving the candidate publisher to Workload Identity Federation would weaken the
first reason considerably but leaves the second one untouched. Do not treat WIF
adoption as grounds for collapsing the staging step.
