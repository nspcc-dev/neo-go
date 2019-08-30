# Builder image
FROM golang:1.12-alpine3.10 as builder

RUN set -x \
    && apk add --no-cache git \
    && mkdir -p /tmp

COPY . /neo-go

WORKDIR /neo-go

ARG REPO=repository
ARG VERSION=dev

# https://github.com/golang/go/wiki/Modules#how-do-i-use-vendoring-with-modules-is-vendoring-going-away
# go build -mod=vendor
RUN set -x \
    && export GOGC=off \
    && export GO111MODULE=on \
    && export CGO_ENABLED=0 \
    && export LDFLAGS="-X ${REPO}/config.Version=${VERSION}" \
    && go mod tidy -v \
    && go mod vendor \
    && go build -v -mod=vendor -ldflags "${LDFLAGS}" -o /go/bin/neo-go ./cli/main.go

# Executable image
FROM alpine:3.10

ENV   NETMODE=testnet
ARG   VERSION
LABEL version=$VERSION

WORKDIR /

ENV NETMODE=testnet
COPY --from=builder /neo-go/config   /config
COPY --from=builder /go/bin/neo-go     /usr/bin/neo-go
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["/usr/bin/neo-go"]

CMD ["node", "--config-path", "/config", "--testnet"]
