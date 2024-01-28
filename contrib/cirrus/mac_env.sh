#!/bin/bash

set -euo pipefail

# This script is intended to be called by Cirrus-CI on a Mac M1 persistent worker.
# It runs /after/ `mac_setup.sh` to help developers debug any environment
# related issues.  It must not make any actually changes to the environment.

# Many variables can affect operations, make them all known to assist debugging.
echo "Selection of current env. vars:"
for env_var_name in $(awk 'BEGIN{for(v in ENVIRON) print v}' | grep -Eiv '(^PATH$)|(^BASH_FUNC)|(^_.*)' | sort)
do
    echo "    ${env_var_name}=${!env_var_name}"
done

# The latest toolchain is always installed when instances are created.  Make it known
# what version that actually is.
go version

# Golang is sensitive to a collection of key variables.  Make them known to assist
# with any debugging.  N/B: Most filepath values should point somewhere under $HOME/ci/
go env

# The latest version is installed system-wide when instances are created. Make the
# current version known.
vfkit --version
