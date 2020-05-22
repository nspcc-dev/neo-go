#!/bin/sh

BIN=/usr/bin/neo-go

if [ -z "$ACC"]; then
  ACC=/6000-privnet-blocks.acc.gz
fi

case $@ in
  "node"*)
  echo "=> Try to restore blocks before running node"
  if test -f $ACC; then
    gunzip --stdout /$ACC > /privnet.acc
    ${BIN} db restore -p --config-path /config -i /privnet.acc
  fi
    ;;
esac

${BIN} "$@"
