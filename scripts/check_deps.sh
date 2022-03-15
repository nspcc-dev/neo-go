#!/bin/sh

die() {
	echo "$*"
	exit 1
}

find -name go.mod -print0 |
	xargs -0 -n1 grep -o 'pkg/interop v\S*' |
	uniq | wc -l |
	xargs -I{} -n1 [ 1 -eq {} ] ||
	die "Different versions for dependencies in go.mod"

INTEROP_COMMIT="$(sed -E -n -e 's/.*pkg\/interop.+-.+-(\w+)/\1/ p' go.mod)"
git merge-base --is-ancestor "$INTEROP_COMMIT" HEAD ||
	die "pkg/interop commit $INTEROP_COMMIT was not found in git"

for dir in examples/*/; do
	INTEROP_COMMIT="$(sed -E -n -e 's/.*pkg\/interop.+-.+-(\w+)/\1/ p' "$dir/go.mod")"
	git merge-base --is-ancestor "$INTEROP_COMMIT" HEAD ||
		die "$dir: pkg/interop commit $INTEROP_COMMIT was not found in git"

	if [ -z "${dir#*nft-nd-nns/}" ]; then
		NEO_GO_COMMIT="$(sed -E -n -e 's/.*neo-go\s.+-.+-(\w+)/\1/ p' "$dir/go.mod")"
		git merge-base --is-ancestor "$NEO_GO_COMMIT" HEAD ||
			die "$dir: neo-go commit $NEO_GO_COMMIT was not found in git"
	fi
done
