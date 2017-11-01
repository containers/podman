#!/bin/bash

# example usage
#   $ ./venv-ansible-playbook.sh \
#               -i 192.168.169.170 \
#               --private-key=/path/to/key \
#               --extra-vars "pullrequest=42" \
#               --extra-vars "commit=abcd1234" \
#               --user root \
#               --verbose \
#               $PWD/crio-integration-playbook.yaml

# All errors are fatal
set -e

SCRIPT_PATH=`realpath $(dirname $0)`
REQUIREMENTS="$SCRIPT_PATH/requirements.txt"

echo

if ! type -P virtualenv &> /dev/null
then
    echo "Could not find required 'virtualenv' binary installed on system."
    exit 1
fi

if [ "$#" -lt "1" ]
then
    echo "No ansible-playbook command-line options specified."
    echo "usage: $0 -i whatever --private-key=something --extra-vars foo=bar playbook.yml"
    exit 2
fi

# Avoid dirtying up repository, keep execution bits confined to a known location
if [ -z "$WORKSPACE" ] || [ ! -d "$WORKSPACE" ]
then
    export WORKSPACE="$(mktemp -d)"
    echo "Using temporary \$WORKSPACE=\"$WORKSPACE\" for execution environment."
    echo "Directory will be removed upon exit.  Export this variable with path"
    echo "to an existing directory to preserve contents."
    trap 'rm -rf "$WORKSPACE"' EXIT
else
    echo "Using existing \$WORKSPACE=\"$WORKSPACE\" for execution environment."
    echo "Directory will be left as-is upon exit."
    # Don't recycle cache, next job may have different requirements
    trap 'rm -rf "$PIPCACHE"' EXIT
fi

# Create a directory to contain logs and test artifacts
export ARTIFACTS=$(mkdir -pv $WORKSPACE/artifacts | tail -1 | cut -d \' -f 2)
[ -d "$ARTIFACTS" ] || exit 3

# All command failures from now on are fatal
set -e
echo
echo "Bootstrapping trusted virtual environment, this may take a few minutes, depending on networking."
echo "(logs: \"$ARTIFACTS/crio_venv_setup_log.txt\")"
echo


(
    set -x
    cd "$WORKSPACE"
    # When running more than once, make it fast by skipping the bootstrap
    if [ ! -d "./.cri-o_venv" ]; then
        # N/B: local system's virtualenv binary - uncontrolled version fixed below
        virtualenv --no-site-packages --python=python2.7 ./.venvbootstrap
        # Set up paths to install/operate out of $WORKSPACE/.venvbootstrap
        source ./.venvbootstrap/bin/activate
        # N/B: local system's pip binary - uncontrolled version fixed below
        # pip may not support --cache-dir, force it's location into $WORKSPACE the ugly-way
        OLD_HOME="$HOME"
        export HOME="$WORKSPACE"
        export PIPCACHE="$WORKSPACE/.cache/pip"
        pip install --force-reinstall --upgrade pip==9.0.1
        # Undo --cache-dir workaround
        export HOME="$OLD_HOME"
        # Install fixed, trusted, hashed versions of all requirements (including pip and virtualenv)
        pip --cache-dir="$PIPCACHE" install --require-hashes \
            --requirement "$SCRIPT_PATH/requirements.txt"

        # Setup trusted virtualenv using hashed binary from requirements.txt
        ./.venvbootstrap/bin/virtualenv --no-site-packages --python=python2.7 ./.cri-o_venv
        # Exit untrusted virtualenv
        deactivate
    fi
    # Enter trusted virtualenv
    source ./.cri-o_venv/bin/activate
    # Upgrade stock-pip to support hashes
    pip install --force-reinstall --cache-dir="$PIPCACHE" --upgrade pip==9.0.1
    # Re-install from cache but validate all hashes (including on pip itself)
    pip --cache-dir="$PIPCACHE" install --require-hashes \
        --requirement "$SCRIPT_PATH/requirements.txt"
    # Remove temporary bootstrap virtualenv
    rm -rf ./.venvbootstrap
    # Exit trusted virtualenv

) &> $ARTIFACTS/crio_venv_setup_log.txt;

echo
echo "Executing \"$WORKSPACE/.cri-o_venv/bin/ansible-playbook $@\""
echo

# Execute command-line arguments under virtualenv
source ${WORKSPACE}/.cri-o_venv/bin/activate
${WORKSPACE}/.cri-o_venv/bin/ansible-playbook $@
