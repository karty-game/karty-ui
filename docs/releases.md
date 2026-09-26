# KartUI releases

Public `vVERSION` tags make the Go compiler/schema packages available as a
versioned Go module and trigger the Release KartUI workflow.

The workflow tests and lints the Go packages, runs the pinned editor tests,
builds a VSIX, and uses GoReleaser to attach it to a GitHub release. It uses only
this repository's `GITHUB_TOKEN`; no private engine access is required.

The editor version comes from `editors/vscode/package.json`. It can evolve
independently of SDK versions. Language changes requiring new runtime behavior
must still be coordinated with an engine SDK release.

The separate Package editor workflow produces an installable VSIX as a workflow
artifact without publishing a release. GoReleaser does not build an executable
for this library repository and does not upload additional source archives.

Initial module publication order: KartUI and Karty SDK `v0.0.1`, then Karty `v0.0.1`.
Resolve and commit the public dependency checksums before tagging dependent
repositories. These initial module tags have not been published by this migration;
local ignored workspaces currently supply the dependencies.
