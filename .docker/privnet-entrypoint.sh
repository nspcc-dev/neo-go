#!/bin/sh

BIN=/usr/bin/neo-go

case $@ in
"node"*)
	echo "=> Try to restore blocks before running node"
	if [ -f "$ACC" ]; then
		gunzip --stdout "$ACC" >/privnet.acc
		${BIN} db restore -p --config-path /config -i /privnet.acc
	fi
	;;
esac

${BIN} "$@"
