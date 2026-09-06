# Versions use exact commits and existing release infrastructure

Version publication uses images for the exact main commit selected at dispatch,
even for changelog-only changes. Verified images for that SHA are reused; missing
targets are built through reusable CI with the same quality checks, signer and
staging/attestation guarantees as main CI. A successful image build alone is not
a successful quality check.

Prepare all prod images and SHA tags before creating a lightweight Git tag, then
publish version image tags. Before the Git tag exists, a new request can reuse
the version number with the then-current main SHA. After it exists, recovery uses
its commit and existing verified SHA images; missing or conflicting content fails
instead of rebuilding or moving the tag. Annotated tags are peeled to their commit
without interpreting annotation text. No custom digest record is maintained.

Use the existing prod Environment, release credential and approval gate. Separate
version publication from deployment through workflow behavior, without introducing
another Environment, account or secret. Only final publication holds the shared
release lock; checks/builds remain outside it. This accepts prod approval and
operator investigation when existing image state cannot support recovery in
exchange for fewer credentials, configuration surfaces and persistence contracts.

Deployment version selection also uses the exact tag commit. Older tags that
relied on historical candidate selection can still be deployed by an explicit
candidate SHA. Dev latest selection remains unchanged.
