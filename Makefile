BRANCH = "master"
BUILD_TIME = "$(shell date -u +\"%Y-%m-%dT%H:%M:%SZ\")"
VERSION = $(shell cat ./VERSION)
REPONAME = "neo-go"
NETMODE ?= "privnet"

build-linux:
	@GOOS=linux go build -ldflags "-X github.com/CityOfZion/neo-go/config.Version=${VERSION}-dev -X github.com/CityOfZion/neo-go/config.BuildTime=${BUILD_TIME}" -o ./bin/neo-go ./cli/main.go

build:
	@go build -ldflags "-X github.com/CityOfZion/neo-go/config.Version=${VERSION}-dev -X github.com/CityOfZion/neo-go/config.BuildTime=${BUILD_TIME}" -o ./bin/neo-go ./cli/main.go

build-image:
	docker build -t cityofzion/neo-go .

check-version:
	git fetch && (! git rev-list ${VERSION})

deps:
	@dep ensure

push-tag:
	git checkout ${BRANCH}
	git pull origin ${BRANCH}
	git tag ${VERSION}
	git push origin ${VERSION}

push-to-registry:
	@docker login -e ${DOCKER_EMAIL} -u ${DOCKER_USER} -p ${DOCKER_PASS}
	@docker tag CityOfZion/${REPONAME}:latest CityOfZion/${REPONAME}:${CIRCLE_TAG}
	@docker push CityOfZion/${REPONAME}:${CIRCLE_TAG}
	@docker push CityOfZion/${REPONAME}

run: build
	./bin/neo-go node -config-path ./config -${NETMODE}

test:
	@go test ./... -cover

vet:
	@go vet ./...
