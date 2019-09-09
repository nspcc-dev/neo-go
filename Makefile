BRANCH = "master"
REPONAME = "neo-go"
NETMODE ?= "privnet"
BINARY = "./bin/neo-go"

REPO ?= "$(shell go list -m)"
VERSION ?= "$(shell git describe --tags 2>/dev/null | sed 's/^v//')"
BUILD_FLAGS = "-X $(REPO)/config.Version=$(VERSION)"

# All of the targets are phony here because we don't really use make dependency
# tracking for files
.PHONY: build deps image check-version clean-cluster push-tag push-to-registry \
	run run-cluster test vet lint fmt cover

build: deps
	@echo "=> Building binary"
	@set -x \
		&& export GOGC=off \
		&& export CGO_ENABLED=0 \
		&& go build -v -mod=vendor -ldflags $(BUILD_FLAGS) -o ${BINARY} ./cli/main.go

image:
	@echo "=> Building image"
	@docker build -t cityofzion/neo-go:latest --build-arg REPO=$(REPO) --build-arg VERSION=$(VERSION) .
	@docker build -t cityofzion/neo-go:$(VERSION) --build-arg REPO=$(REPO) --build-arg VERSION=$(VERSION) .

check-version:
	git fetch && (! git rev-list ${VERSION})

clean-cluster:
	@echo "=> Removing all containers and chain storage"
	@rm -rf chains/privnet-docker-one chains/privnet-docker-two chains/privnet-docker-three chains/privnet-docker-four
	@docker-compose stop
	@docker-compose rm -f

deps:
	@go mod tidy -v
	@go mod vendor

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
	${BINARY} node -config-path ./config -${NETMODE}

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

lint:
	@go list ./... | xargs -L1 golint -set_exit_status

fmt:
	@gofmt -l -w -s $$(find . -type f -name '*.go'| grep -v "/vendor/")

cover:
	@go test -v -race ./... -coverprofile=coverage.txt -covermode=atomic
	@go tool cover -html=coverage.txt -o coverage.html
