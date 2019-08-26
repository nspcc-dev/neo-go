BRANCH = "master"
BUILD_TIME = "$(shell date -u +\"%Y-%m-%dT%H:%M:%SZ\")"
REPONAME = "neo-go"
NETMODE ?= "privnet"

REPO ?= "$(shell go list -m)"
VERSION ?= "$(shell git describe --tags 2>/dev/null | sed 's/^v//')"
BUILD_FLAGS = "-X $(REPO)/config.Version=$(VERSION) -X $(REPO)/config.BuildTime=$(BUILD_TIME)"

build: deps
	@echo "=> Building binary"
	@set -x \
		&& export GOGC=off \
		&& export CGO_ENABLED=0 \
		&& echo $(VERSION)-$(BUILD_TIME) \
		&& go build -v -mod=vendor -ldflags $(BUILD_FLAGS) -o ./bin/node ./cli/main.go

image: deps
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
