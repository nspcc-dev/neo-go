# Builder image
FROM golang:1-alpine as builder

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
    && go build -v -mod=vendor -ldflags "${LDFLAGS}" -o /go/bin/neo-go ./cli

# Executable image
FROM alpine

ARG   VERSION
LABEL version=$VERSION

WORKDIR /

COPY --from=builder /neo-go/config /config
COPY --from=builder /neo-go/.docker/6000-privnet-blocks.acc.gz /6000-privnet-blocks.acc.gz
COPY --from=builder /neo-go/.docker/1600-privnet-blocks-single.acc.gz /1600-privnet-blocks-single.acc.gz
COPY --from=builder /neo-go/.docker/privnet-entrypoint.sh /usr/bin/privnet-entrypoint.sh
COPY --from=builder /go/bin/neo-go /usr/bin/neo-go
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["/usr/bin/privnet-entrypoint.sh"]

CMD ["node", "--config-path", "/config", "--privnet"]
