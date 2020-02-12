# #!/usr/bin/env bash
# #
# # This library holds miscellaneous utility functions. If there begin to be groups of functions in this
# # file that share intent or are thematically similar, they should be split into their own files.

# OS_ORIGINAL_WD is the original working directory the script sourcing this utility file was called
# from. This is an important directory as if $0 is a relative path, we cannot use the following path
# utility without knowing from where $0 is relative.
if [[ -z "${OS_ORIGINAL_WD:-}" ]]; then
	# since this could be sourced in a context where the utilities are already loaded,
	# we want to ensure that this is re-entrant, so we only set $OS_ORIGINAL_WD if it
	# is not set already
	OS_ORIGINAL_WD="$( pwd )"
	readonly OS_ORIGINAL_WD
	export OS_ORIGINAL_WD
fi

# os::util::repository_relative_path returns the relative path from the $OS_ROOT directory to the
# given file, if the file is inside of the $OS_ROOT directory. If the file is outside of $OS_ROOT,
# this function will return the absolute path to the file
#
# Globals:
#  - OS_ROOT
# Arguments:
#  - 1: the path to relativize
# Returns:
#  None
function os::util::repository_relative_path() {
	local filename=$1
	local directory; directory="$( dirname "${filename}" )"
	filename="$( basename "${filename}" )"

	if [[ "${directory}" != "${OS_ROOT}"* ]]; then
		pushd "${OS_ORIGINAL_WD}" >/dev/null 2>&1
		directory="$( os::util::absolute_path "${directory}" )"
		popd >/dev/null 2>&1
	fi

	directory="${directory##*${OS_ROOT}/}"

	echo "${directory}/${filename}"
}
readonly -f os::util::repository_relative_path