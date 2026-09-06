# NomNom-Radar

A backend for mobile-vendor and market discovery: consumers find vendors and
vendor clusters, and vendors notify subscribed consumers when they are nearby.

This glossary exists because several words in this repository mean more than
one thing depending on where you are standing, and each collision has already
produced a real defect.

## Release and delivery

**Target**:
One deployable unit in the release catalog — `radar`, `geoworker`, or
`device-cleanup`. A target is a build stage, an image repository, and a Cloud
Run resource under one name.
_Avoid_: service (a target may be a Cloud Run Job), component, unit

**Target environment**:
`dev` or `prod`. Always name it in full, including in variables. A bare
`target` never means an environment.
_Avoid_: env, stage, deployment target

**Candidate**:
A `main` commit for which every target has a complete, attested image.
Candidacy is a property of a commit, not of a build.
_Avoid_: build, artifact, release candidate

**Needs candidate**:
The decision that a commit requires new images, either because a push changed
a release-impacting path or a new version request lacks images for that exact
commit. This is a boolean about work to do, not the noun above.
_Avoid_: candidate (as a boolean)

**Release SHA**:
The candidate commit selected for deployment or version preservation. Also the value of the
`release-sha` Cloud Run label on every target.
_Avoid_: version, deployed SHA

**Control SHA**:
The `main` commit that supplied the running workflow definition. Automation
always executes from this commit; it is not necessarily the release SHA.
_Avoid_: workflow SHA, current SHA

**Impact path**:
A repository path whose change requires new candidate images.
_Avoid_: watched path, trigger path

**Version**:
An operator-assigned stable name for fixed release content and its changelog.
A version does not imply that production deployment succeeded.
_Avoid_: deployed version, production release

**Promotion**:
Copying an exact dev digest into the prod registry without rebuilding.
Promotion never produces a new image.
_Avoid_: prod build, republish, deploy to prod

**Release plan**:
The fixed candidate, target environment, and image digests selected for a
release attempt, retained as the basis for retrying that attempt.
_Avoid_: manifest (the deployment template), baseline, bundle

## Review

Two unrelated controls are both called review. Never use the bare word.

**Merge review**:
The `main` ruleset's pull-request approval requirement, which decides what
reaches `main`. Repository admins bypass it.
_Avoid_: review, approval, PR gate

**Deployment review**:
The `prod` GitHub Environment's required-reviewers rule, which decides whether
a dispatched prod run may proceed. It is unrelated to pull requests, and with a
single maintainer it is a confirmation pause rather than separation of duties.
_Avoid_: review, approval, prod gate
