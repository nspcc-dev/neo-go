BRANCH = "master"
BUILD_TIME = "$(shell date -u +\"%Y-%m-%dT%H:%M:%SZ\")"
VERSION = $(shell cat ./VERSION)
REPONAME = "neo-go"
NETMODE ?= "privnet"

build:
	@echo "=> Building darwin binary"
	@go build -i -ldflags "-X github.com/CityOfZion/neo-go/config.Version=${VERSION}-dev -X github.com/CityOfZion/neo-go/config.BuildTime=${BUILD_TIME}" -o ./bin/neo-go ./cli/main.go

build-image:
	docker build -t cityofzion/neo-go --build-arg VERSION=${VERSION} .

build-linux:
	@echo "=> Building linux binary"
	@GOOS=linux go build -i -ldflags "-X github.com/CityOfZion/neo-go/config.Version=${VERSION}-dev -X github.com/CityOfZion/neo-go/config.BuildTime=${BUILD_TIME}" -o ./bin/neo-go ./cli/main.go

check-version:
	git fetch && (! git rev-list ${VERSION})

clean-cluster:
	@echo "=> Removing all containers and chain storage"
	@rm -rf chains/privnet-docker-one chains/privnet-docker-two chains/privnet-docker-three chains/privnet-docker-four
	@docker-compose stop
	@docker-compose rm -f

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

run-cluster: build-linux
	@echo "=> Starting docker-compose cluster"
	@echo "=> Building container image"
	@docker-compose build
	@docker-compose up -d
	@echo "=> Tailing logs, exiting this prompt will not stop the cluster"
	@docker-compose logs -f

test:
	@go test ./... -cover

vet:
	@go vet ./...
