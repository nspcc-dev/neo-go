#!/bin/sh

if [ -z "$1" ]; then
	echo "Usage: ./update_deps.sh <revision>"
	exit 1
fi

REV="$1"
root="$(git rev-parse --show-toplevel)"

cd "$root" || exit 1
go get github.com/nspcc-dev/neo-go/pkg/interop@"$REV"
go mod tidy

for dir in "$root"/examples/*/; do
	cd "$dir" || exit 1
	go get github.com/nspcc-dev/neo-go/pkg/interop@"$REV"
	go mod tidy
done

cd "$root"/internal/contracts/oracle_contract || exit 1
go get github.com/nspcc-dev/neo-go/pkg/interop@"$REV"
go mod tidy
