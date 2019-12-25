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

### Integration benchmark test
#### Script run
Use `./runIntegration.sh`

#### Manual run
This test requires to have:
- privatenet running
- node running

To run privatenet use this command `$make env_up`.
There are several nodes could be used (neo-go, neo-sharp, ...)
- For neo-go node use docker image `nspccdev/neo-go-integration` which is built exactly for running with local privatenet.
- For C# node use use docker image `nspccdev/neo-sharp-integration` 
To run node: 

neo-go:
`$docker run --rm --network neo_go_network -p 20331-20332:20331-20332 -p 2112:2112 --name neo-go-integraion --ip 172.200.0.5 -t nspccdev/neo-go-integration
`
neo-sharp:
`$docker run --rm --network neo_go_network -p 20331-20332:20331-20332 -p 2112:2112 --name neo-sharp-integraion --ip 172.200.0.5 -t nspccdev/neo-sharp-integration
`

After that use benchmark integration test `integration_test.go`.

#### Update integration docker image
To update nspccdev/neo-go-integration docker image there are several steps to be done:

Neo-go:
1. Change `protocol.privnet.yml`:
```
  SeedList:
    - 172.200.0.1:20333
    - 172.200.0.2:20334
    - 172.200.0.3:20335
    - 172.200.0.4:20336
```
Or use `protocol.privnet.integration.yml` by renaming it to `protocol.privnet.yml`.
2. Build image `$ docker build -t nspccdev/neo-go-integration:0.70.1 .` where 0.70.1 is current release version.
3. Push image `$  docker push nspccdev/neo-go-integration:0.70.1`
