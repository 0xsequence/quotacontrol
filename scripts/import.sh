#!/bin/sh

# This script imports error codes from the remote RIDL file.
# It uses the go.mod file to determine the version of the package.
# Sample usage in makefile:
#
# import-errors:
#	curl https://raw.githubusercontent.com/0xsequence/quotacontrol/refs/heads/master/scripts/import.sh | [PKG=package] sh;

PKG="${PKG:-"0xsequence/quotacontrol"}"

echo $PKG

VERSION=$(cat go.mod | grep -v "//" | grep $PKG | awk '{print$ 2}')
REF="tags"
COMMIT=$(echo $VERSION | cut -d'-' -f 3)
if [[ "$VERSION" != "$COMMIT" ]]; then
    REF="heads"
    VERSION="$COMMIT"
fi

curl -s "https://raw.githubusercontent.com/$PKG/refs/$REF/$VERSION/proto/errors.ridl" > ./proto/$(basename "$PKG").errors.ridl
