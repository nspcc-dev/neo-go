# Release instructions

This document outlines the neo-go release process. It can be used as a todo
list for a new release.

## Check the state

These should run successfully:
 * build
 * unit-tests
 * lint
 * privnet with consensus nodes
 * mainnet synchronization

## Update CHANGELOG and ROADMAP

Add an entry to the CHANGELOG.md following the style established there. Add a
codename, version and release date in the heading. Write a paragraph
describing the most significant changes done in this release. Then, add
sections with new features implemented and bugs fixed describing each change in detail and
with a reference to Github issues. Add generic improvements section for
changes that are not directly visible to the node end-user, such as performance
optimizations, refactoring and API changes. Add a "Behaviour changes" section
if there are any incompatible changes in default settings or the way commands
operate.

Update ROADMAP.md if necessary.

Create a PR with CHANGELOG/ROADMAP changes, review/merge it.

## Create a Github release and a tag

Use "Draft a new release" button in the "Releases" section. Create a new
`vX.Y.Z` tag for it following the semantic versioning standard. Put change log
for this release into the description. Do not attach any binaries at this step.

## Add automatically-built binaries

New release created at the previous step triggers automatic builds (if not,
start them manually from the Build Github workflow), so wait for them to
finish. Then download currently supported binaries (at the time of writing
that's `neo-go-darwin-arm64`, `neo-go-linux-amd64`, `neo-go-linux-arm64` and
`neo-go-windows-amd64`), unpack archives and add resulting binaries (named in
the same way as archives) to the previously created release.

## Close Github milestone

Close corresponding X.Y.Z Github milestone.

## Announcements

Copy the github release page link to:
 * Discord channel
 * Riot channel

## Deployment

Deploy the updated version to the mainnet/testnet.
