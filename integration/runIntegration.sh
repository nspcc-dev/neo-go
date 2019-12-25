#!/bin/sh
cd ..
make env_up
docker run --rm --network neo_go_network -p 20331-20332:20331-20332 -p 2112:2112 --name neo-go-integration --ip 172.200.0.5 -t nspccdev/neo-go-integration:0.70.1 &
(sleep 5s
cd integration || exit
go test -run TestIntegration
)
docker stop neo-go-integration
make env_down
