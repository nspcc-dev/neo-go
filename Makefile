BRANCH = "master"
REPONAME = "neo-go"
NETMODE ?= "privnet"
BINARY=neo-go
BINARY_PATH=./bin/$(BINARY)$(shell go env GOEXE)
GO_VERSION ?= 1.23
DESTDIR = ""
SYSCONFIGDIR = "/etc"
BINDIR = "/usr/bin"
SYSTEMDUNIT_DIR = "/lib/systemd/system"
UNITWORKDIR = "/var/lib/neo-go"

IMAGE_SUFFIX="$(shell if [ "$(OS)" = Windows_NT ]; then echo "_WindowsServerCore"; fi)"
D_FILE ?= "$(shell if [ "$(OS)" = Windows_NT ]; then echo "Dockerfile.wsc"; else echo "Dockerfile"; fi)"
DC_FILE ?= ".docker/docker-compose.yml" # Single docker-compose for Ubuntu/WSC, should be kept in sync with ENV_IMAGE_TAG.
ENV_IMAGE_TAG="env_neo_go_image"

REPO ?= "$(shell go list -m)"
VERSION ?= "$(shell git describe --tags --match "v*" --abbrev=8 2>/dev/null | sed -r 's,^v([0-9]+\.[0-9]+)\.([0-9]+)(-.*)?$$,\1 \2 \3,' | while read mm patch suffix; do if [ -z "$$suffix" ]; then echo $$mm.$$patch; else patch=`expr $$patch + 1`; echo $$mm.$${patch}-pre$$suffix; fi; done)"
MODVERSION ?= "$(shell cat go.mod | cat go.mod | sed -r -n -e 's|.*pkg/interop (.*)|\1|p')"
BUILD_FLAGS = "-X '$(REPO)/pkg/config.Version=$(VERSION)' -X '$(REPO)/cli/smartcontract.ModVersion=$(MODVERSION)'"

IMAGE_REPO=nspccdev/neo-go

DISABLE_NEOTEST_COVER=1

ROOT_DIR:=$(dir $(realpath $(firstword $(MAKEFILE_LIST))))
GOMODDIRS=$(dir $(shell find $(ROOT_DIR) -name go.mod))

# All of the targets are phony here because we don't really use make dependency
# tracking for files
.PHONY: build $(BINARY) deps image docker/$(BINARY) image-latest image-push image-push-latest clean-cluster \
	test vet lint fmt cover version gh-docker-vars

build: deps
	@echo "=> Building binary"
	@set -x \
		&& export GOGC=off \
		&& export CGO_ENABLED=0 \
		&& go build -trimpath -v -ldflags $(BUILD_FLAGS) -o ${BINARY_PATH} ./cli/main.go

$(BINARY): build

docker/$(BINARY):
	@echo "=> Building binary using clean Docker environment"
	@docker run --rm -t \
	-v `pwd`:/src \
	-w /src \
	-u "$$(id -u):$$(id -g)" \
	--env HOME=/src \
	golang:$(GO_VERSION) make $(BINARY)

neo-go.service: neo-go.service.template
	@sed -r -e 's_BINDIR_$(BINDIR)_' -e 's_UNITWORKDIR_$(UNITWORKDIR)_' -e 's_SYSCONFIGDIR_$(SYSCONFIGDIR)_' $< >$@

install: build neo-go.service
	@echo "=> Installing systemd service"
	@mkdir -p $(DESTDIR)$(SYSCONFIGDIR)/neo-go \
		&& mkdir -p $(SYSTEMDUNIT_DIR) \
		&& cp ./neo-go.service $(SYSTEMDUNIT_DIR) \
		&& cp ./config/protocol.mainnet.yml $(DESTDIR)$(SYSCONFIGDIR)/neo-go \
		&& cp ./config/protocol.privnet.yml $(DESTDIR)$(SYSCONFIGDIR)/neo-go \
		&& cp ./config/protocol.testnet.yml $(DESTDIR)$(SYSCONFIGDIR)/neo-go \
		&& install -m 0755 -t $(BINDIR) $(BINARY_PATH) \

postinst: install
	@echo "=> Preparing directories and configs"
	@id neo-go || useradd -s /usr/sbin/nologin -d $(UNITWORKDIR) neo-go \
		&& mkdir -p $(UNITWORKDIR) \
		&& chown -R neo-go:neo-go $(UNITWORKDIR) $(BINDIR)/neo-go \
		&& systemctl enable neo-go.service

image: deps
	@echo "=> Building image"
	@echo "   Dockerfile: $(D_FILE)"
	@echo "   Tag: $(IMAGE_REPO):$(VERSION)$(IMAGE_SUFFIX)"
	@docker build -f $(D_FILE) -t $(IMAGE_REPO):$(VERSION)$(IMAGE_SUFFIX) --build-arg REPO=$(REPO) --build-arg VERSION=$(VERSION) .

image-latest: deps
	@echo "=> Building image with 'latest' tag"
	@echo "   Dockerfile: Dockerfile" # Always use default Dockerfile for Ubuntu as `latest`.
	@echo "   Tag: $(IMAGE_REPO):latest"
	@docker build -t $(IMAGE_REPO):latest --build-arg REPO=$(REPO) --build-arg VERSION=$(VERSION) .

image-push:
	@echo "=> Publish image"
	@echo "   Tag: $(IMAGE_REPO):$(VERSION)$(IMAGE_SUFFIX)"
	@docker push $(IMAGE_REPO):$(VERSION)$(IMAGE_SUFFIX)

image-push-latest:
	@echo "=> Publish image for Ubuntu with 'latest' tag"
	@docker push $(IMAGE_REPO):latest

deps:
	@CGO_ENABLED=0 \
	go mod download
	@CGO_ENABLED=0 \
	go mod tidy -v

version:
	@echo $(VERSION)

gh-docker-vars:
	@echo "file=$(D_FILE)"
	@echo "version=$(VERSION)"
	@echo "repo=$(IMAGE_REPO)"
	@echo "suffix=$(IMAGE_SUFFIX)"

test:
	@go test ./... -cover

vet:
	@go vet ./...

.golangci.yml:
	curl -L -o $@ https://github.com/nspcc-dev/.github/raw/master/.golangci.yml

lint: .golangci.yml
	@for dir in $(GOMODDIRS); do \
		(cd "$$dir" && golangci-lint run --config $(ROOT_DIR)/$< | sed -r "s,^,$$dir," | sed -r "s,^$(ROOT_DIR),,") \
	done

fmt:
	@gofmt -l -w -s $$(find . -type f -name '*.go'| grep -v "/vendor/")

cover:
	@go test -v -race ./... -coverprofile=coverage.txt -covermode=atomic -coverpkg=./pkg/...,./cli/...
	@go tool cover -html=coverage.txt -o coverage.html

# --- Ubuntu/Windows environment ---
env_image:
	@echo "=> Building env image"
	@echo "   Dockerfile: $(D_FILE)"
	@echo "   Tag: $(ENV_IMAGE_TAG)"
	@docker build \
		-f $(D_FILE) \
		-t $(ENV_IMAGE_TAG) \
		--build-arg REPO=$(REPO) \
		--build-arg VERSION=$(VERSION) .

env_up:
	@echo "=> Bootup environment"
	@echo "   Docker-compose file: $(DC_FILE)"
	@docker compose -f $(DC_FILE) up -d node_one node_two node_three node_four

env_single:
	@echo "=> Bootup environment"
	@docker compose -f $(DC_FILE) up -d node_single

env_down:
	@echo "=> Stop environment"
	@docker compose -f $(DC_FILE) down

env_restart:
	@echo "=> Stop and start environment"
	@docker compose -f $(DC_FILE) restart

env_clean: env_down
	@echo "=> Cleanup environment"
	@docker volume rm docker_volume_chain
