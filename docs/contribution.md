# Contributions


This document will state guidelines for submitting code.

## Pre-requisites

In order to contribute to Golang, it is essential that you know Golang or language similar to Golang. It is also required that you are familiar with NEO infrastructure. 

Reading:

    http://docs.neo.org/en-us/network/network-protocol.html

    https://tour.golang.org/welcome/1

    https://golang.org/doc/effective_go.html#introduction

## 1. Open an Issue Prior

Please open an issue prior to working on the feature, stating the feature you would like to work on.

## 2. Unit test

Unit tests are important, when submitting a PR please ensure that you have at least 70% test coverage.

## 3. Avoid CGo

Due to the complications that come with using CGo, if using it, please provide justification before using it. If using CGo, can provide significant speed boosts and or a large part of the code can be processed in CGo, then we should be able to come to consensus on using it. Generally, it is better to write everything in Golang, so that the code can be maintained by other Gophers.