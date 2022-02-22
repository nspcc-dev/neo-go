# Builder image
# Keep go version in sync with Build GA job.
FROM golang:1.18-alpine as builder

# Display go version for information purposes.
RUN go version

RUN set -x \
    && apk add --no-cache git make \
    && mkdir -p /tmp

COPY . /neo-go

WORKDIR /neo-go

ARG REPO=repository
ARG VERSION=dev

RUN make build

# Executable image
FROM alpine

ARG   VERSION
LABEL version=$VERSION

WORKDIR /

COPY --from=builder /neo-go/config /config
COPY --from=builder /neo-go/.docker/privnet-entrypoint.sh /usr/bin/privnet-entrypoint.sh
COPY --from=builder /neo-go/bin/neo-go /usr/bin/neo-go
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["/usr/bin/privnet-entrypoint.sh"]

CMD ["node", "--config-path", "/config", "--privnet"]
