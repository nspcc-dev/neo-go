# Building Debian package on host

## Prerequisites

For now, we're assuming building for Debian 11 (stable) x86_64.

Go version 18.4 or later should already be installed, i.e. this runs
successfully:

* `make all`

## Installing packaging dependencies

```shell
$ sudo apt install debhelper-compat dh-sequence-bash-completion devscripts
```

Warining: number of package installed is pretty large considering dependecies.

## Package building

```shell
$ make debpackage
```

## Leftovers cleaning

```shell
$ make debclean
```
or
```shell
$ dh clean
```

# Package versioning

By default, package version is based on product version and may also contain git
tags and hashes.

Package version could be overwritten by setting `PKG_VERSION` variable before
build, Debian package versioning rules should be respected.

```shell
$ PKG_VERSION=0.32.0 make debpackge
```
