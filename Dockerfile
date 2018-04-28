FROM vidsyhq/go-base:latest
LABEL maintainers="anthdm,stevenjack"

ENV NETMODE=testnet

ARG VERSION
LABEL version=$VERSION

ADD bin/neo-go /usr/bin/neo-go
ADD config /config

RUN chmod u+x /usr/bin/neo-go
RUN mkdir -p /chains

ENTRYPOINT ["neo-go"]
CMD ["node", "--config-path", "./config", "--testnet"]
