BRANCH = "master"
BUILD_TIME = "$(shell date -u +\"%Y-%m-%dT%H:%M:%SZ\")"
VERSION = $(shell cat ./VERSION)
NETMODE ?= "privnet"

build:
	@go build -ldflags "-X github.com/CityOfZion/neo-go/pkg/network.Version=${VERSION}-dev -X github.com/CityOfZion/neo-go/pkg/network.BuildTime=${BUILD_TIME}" -o ./bin/neo-go ./cli/main.go

check-version:
	git fetch && (! git rev-list ${VERSION})

deps:
	@dep ensure

push-tag:
	git checkout ${BRANCH}
	git pull origin ${BRANCH}
	git tag ${VERSION}
	git push origin ${VERSION}

run: build
	./bin/neo-go node -config-path ./config -${NETMODE}

test:
	@go test ./... -cover

vet:
	@go vet ./...
