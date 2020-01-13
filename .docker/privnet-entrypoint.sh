#!/bin/sh
if test -f /privnet-blocks.acc.gz; then
	gunzip /privnet-blocks.acc.gz
	/usr/bin/neo-go db restore -i /privnet-blocks.acc
fi
/usr/bin/neo-go "$@"
