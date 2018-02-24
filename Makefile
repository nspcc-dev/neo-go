BRANCH = "master"
VERSION = $(shell cat ./VERSION)

build:
	@go build -o ./bin/neo-go ./cli/main.go

check-version:
	git fetch && (! git rev-list ${VERSION})

deps:
	@dep ensure

push-tag:
	git checkout ${BRANCH}
	git pull origin ${BRANCH}
	git tag ${VERSION}
	git push origin ${BRANCH} --tags

test:
	@go test ./... -cover

vet:
	@go vet ./...