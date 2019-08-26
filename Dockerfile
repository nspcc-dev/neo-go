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
    && export BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
    && export GOGC=off \
    && export CGO_ENABLED=0 \
    && export LDFLAGS="-X ${REPO}/config.Version=${VERSION} -X ${REPO}/config.BuildTime=${BUILD_TIME}" \
    && go build -v -mod=vendor -ldflags "${LDFLAGS}" -o /go/bin/node ./cli/main.go

# Executable image
FROM alpine:3.10

ENV   NETMODE=testnet
ARG   VERSION
LABEL version=$VERSION

WORKDIR /

ENV NETMODE=testnet
COPY --from=builder /neo-go/config   /config
COPY --from=builder /go/bin/node     /usr/bin/node
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["/usr/bin/node"]

CMD ["node", "--config-path", "./config", "--testnet"]
