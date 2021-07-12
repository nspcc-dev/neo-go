# Contributing

First, thank you for contributing! We love and encourage pull requests from everyone. Please
follow the guidelines:

1. Check the open [issues](https://github.com/nspcc-dev/neo-go/issues) and
[pull requests](https://github.com/nspcc-dev/neo-go/pulls) for existing discussions.
1. Open an issue first, to discuss a new feature or enhancement.
1. Write tests, and make sure the test suite passes locally and on CI.
1. When optimizing something, write benchmarks and attach results:
   ```
   go test -run - -bench BenchmarkYourFeature -count=10 ./... >old // on master
   go test -run - -bench BenchmarkYourFeature -count=10 ./... >new // on your branch
   benchstat old new
   ```
   `benchstat` is described here https://godocs.io/golang.org/x/perf/cmd/benchstat.

1. Open a pull request, and reference the relevant issue(s).
1. Make sure your commits are logically separated and have good comments
   explaining the details of your change. Add a package/file prefix to your
   commit if that's applicable (like 'vm: fix ADD miscalculation on full
   moon').
1. After receiving feedback, amend your commits or add new ones as
   appropriate.
1. **Have fun!**
