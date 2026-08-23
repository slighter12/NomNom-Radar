# Candidate images get their immutable SHA tag only after attestation

CI builds each candidate image to a run-scoped staging tag, resolves its exact
digest, attests that digest, verifies the attestation, and only then adds the
immutable `:<commit-sha>` tag and deletes the staging tag. The obvious
alternative — build and push straight to the SHA tag, then attest — is roughly
twenty lines shorter, and it is what a reader will be tempted to collapse this
back into.

## Why

Two reasons, and the second is the one that is easy to miss.

**A compromised registry credential must not be able to supply a release
input.** `GCP_CANDIDATE_SA_KEY` is a long-lived JSON service account key with
write access to the dev registry, and this repository is public. Without
attestation, anyone holding that key could push an image under a legitimate
commit SHA tag and the resolver would deploy it to prod. Immutable tags alone
do not close this: they prevent overwriting an existing tag, but not creating
one for a commit CI never built — for example a docs-only commit, which is
exactly what a pinned release is able to select.

**An immutable tag cannot be removed, so publishing it before attestation
turns any interruption into a permanently unreleasable commit.** If CI pushes
the SHA tag and then fails before attesting, that digest can never be attested
afterwards — the release path refuses to re-attest an existing digest, by
design. The commit is then bricked, and recovery means quarantining the SHA and
publishing a new release-impacting commit. The staging tag exists so that an
interrupted run leaves behind a throwaway tag instead of a permanent one. This
reason is operational rather than adversarial, and it survives unchanged even
if the credential risk is later removed.

## Consequences

Moving the candidate publisher to Workload Identity Federation would weaken the
first reason considerably but leaves the second one untouched. Do not treat WIF
adoption as grounds for collapsing the staging step.
