#!/bin/sh
if test -f /6000-privnet-blocks.acc.gz; then
	gunzip /6000-privnet-blocks.acc.gz
	/usr/bin/neo-go db restore -i /6000-privnet-blocks.acc
fi
/usr/bin/neo-go "$@"
