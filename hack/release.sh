#!/bin/sh
#
# Cut a libpod release.  Usage:
#
#   $ hack/release.sh <version> <next-version>
#
# For example:
#
#   $ hack/release.sh 1.2.3 1.3.0
#
# for "I'm cutting 1.2.3, and want to use 1.3.0-dev for future work".

VERSION="$1"
NEXT_VERSION="$2"

if test "${NEXT_VERSION}" != "${NEXT_VERSION%-dev}"
then
	echo "The next-version argument '${NEXT_VERSION}' should not end in '-dev'." >&2
	echo "This script will add the -dev suffix as needed internally.  Try:" >&2
	echo "  $0 ${VERSION} ${NEXT_VERSION%-dev}" >&2
	exit 1
fi

DATE=$(date '+%Y-%m-%d')
LAST_TAG=$(git describe --tags --abbrev=0)

write_go_version()
{
	LOCAL_VERSION="$1"
	sed -i "s/^\(var Version = semver.MustParse( *\"\).*/\1${LOCAL_VERSION}\")/" version/version.go
}

write_spec_version()
{
	LOCAL_VERSION="$1"
	sed -i "s/^\(Version: *\).*/\1${LOCAL_VERSION}/" contrib/spec/podman.spec.in
}

write_changelog()
{
	echo "- Changelog for v${VERSION} (${DATE})" >.changelog.txt &&
	git log --no-merges --format='  * %s' "${LAST_TAG}..HEAD" >>.changelog.txt &&
	echo >>.changelog.txt &&
	cat changelog.txt >>.changelog.txt &&
	mv -f .changelog.txt changelog.txt
}

release_commit()
{
	write_go_version "${VERSION}" &&
	write_spec_version "${VERSION}" &&
	write_changelog &&
	git commit -asm "Bump to v${VERSION}"
}

dev_version_commit()
{
	write_go_version "${NEXT_VERSION}-dev" &&
	write_spec_version "${NEXT_VERSION}" &&
	git commit -asm "Bump to v${NEXT_VERSION}-dev"
}

git fetch origin &&
git checkout -b "bump-${VERSION}" origin/master &&
release_commit &&
git tag -s -m "version ${VERSION}" "v${VERSION}" &&
dev_version_commit
