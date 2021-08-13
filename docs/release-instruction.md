# Release instructions

This documents outlines the neo-go release process, it can be used as a todo
list for a new release.

## Pre-release checks

These should run successfuly:
 * build
 * unit-tests
 * vet
 * lint
 * privnet with consensus nodes

## Writing CHANGELOG

Add an entry to the CHANGELOG.md following the style established there. Add a
codename, version and release date in the heading. Write a paragraph
describing the most significant changes done in this release. Then add
sections with new features and bugs fixed describing each change in detail and
with a reference to Github issues. Add generic improvements section for
changes that are not directly visible to the node end-user such as performance
optimizations, refactoring and API changes. Add a "Behaviour changes" section
if there are any incompatible changes in default settings or the way commands
operate.

## Updating ROADMAP

Update ROADMAP.md if necessary.

## Tag the release

Use `vX.Y.Z` tag following the semantic versioning standard.

## Push changes and release tag to Github

This step should bypass the default PR mechanism to get a correct result (so
that releasing requires admin privileges for the project), both the `master`
branch update and tag must be pushed simultaneously like this:

    $ git push origin master v0.70.1

This is important to ensure proper version in a CI-built binary. Wait for
CircleCI to build it (or them if we're to have more than one OS or
architecture), download it and rename to `neo-go-$OS-$ARCH`, at the moment
that should be `neo-go-linux-amd64`.

## Build and push image to DockerHub

Manually trigger "Push images to DockerHub" workflow from master branch for
the release tag.

## Make a proper Github release

Edit an automatically-created release on Github, copy things from changelog
there and attach previously downloaded binary. Make a release.

## Close Github milestone

Close corresponding X.Y.Z Github milestone.

## Announcements

Copy the github release page link to:
 * Discord channel
 * Riot channel

## Deployment

Deploy updated version to the mainnet/testnet.

## Post-release

The first commit after the release must be tagged with `X.Y.Z+1-pre` tag for
proper semantic-versioned builds, so it's good to make some minor
documentation update after the release and push it with this new tag.
