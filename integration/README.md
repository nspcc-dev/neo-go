# Integration package
The main goal is to have integration tests here.

### Performance test
Right now we have `performance_test.go` to measure number of processed TX per second.
In order to run it:
```
$ cd integration
$ go test -bench=. -benchmem
```

Result:
```
10000            402421 ns/op          177370 B/op         90 allocs/op
PASS
ok      github.com/CityOfZion/neo-go/integration        4.360s

```

Which means that in 4.360 seconds neo-go processes 10 000 transactions.