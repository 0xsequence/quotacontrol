#!/bin/sh

# This script imports error codes from the remote RIDL file.
# It uses the go.mod file to determine the version of the package.
# Sample usage in makefile:
#
# import-errors:
#	curl https://raw.githubusercontent.com/0xsequence/quotacontrol/refs/heads/master/scripts/import.sh | sh;

PKG="0xsequence/quotacontrol"
TAG=$(cat go.mod | grep -v "//" | grep $PKG | awk '{print$ 2}')
URL="https://raw.githubusercontent.com/$PKG/refs/tags/$TAG/proto/quotacontrol.ridl"

COMMIT=$(echo $TAG | cut -d'-' -f 3)
if [[ "$TAG" != "$COMMIT" ]]; then
    URL="https://raw.githubusercontent.com/$PKG/refs/heads/$COMMIT/proto/quotacontrol.ridl";
fi

printf "# $PKG $TAG \n";
curl -s $URL | grep "^error 1";
printf "\n"
