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
A `main` commit for which every target has a complete, attested image in the
dev registry. Candidacy is a property of a commit, not of a build.
_Avoid_: build, artifact, release candidate

**Needs candidate**:
The decision that a push changed a release-impacting path and must therefore
publish new images. This is a boolean about work to do, not the noun above.
_Avoid_: candidate (as a boolean)

**Release SHA**:
The candidate commit a release is deploying. Also the value of the
`release-sha` Cloud Run label on every target.
_Avoid_: version, deployed SHA

**Control SHA**:
The `main` commit that supplied the running workflow definition. Automation
always executes from this commit; it is not necessarily the release SHA.
_Avoid_: workflow SHA, current SHA

**Baseline**:
The release SHA a target environment's fleet is running before this release.
Migration selection is the diff between the baseline and the release SHA.
_Avoid_: previous release, last deploy, current version

**Pinned release**:
A release that names its release SHA directly instead of resolving the newest
compatible ancestor. Used for rollback. It runs no migrations.
_Avoid_: rollback (a pin may also move forward), manual release

**Impact path**:
A repository path whose change requires new candidate images. The list is
`impact_path_args()` in `release.sh` and nothing else.
_Avoid_: watched path, trigger path

**Promotion**:
Copying an exact dev digest into the prod registry without rebuilding.
Promotion never produces a new image.
_Avoid_: prod build, republish, deploy to prod

**Bundle**:
The run-local JSON mapping a release SHA to one exact digest per target. It
exists only for the duration of a workflow run.
_Avoid_: manifest (that word is the rendered Cloud Run YAML), lockfile

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
