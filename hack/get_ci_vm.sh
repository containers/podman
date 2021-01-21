#!/usr/bin/env bash

#
# For help and usage information, simply execute the script w/o any arguments.
#
# This script is intended to be run by podman developers who need to debug
# problems specifically related to Cirrus-CI automated testing.  However,
# because it's only loosely coupled to the `.cirrus.yml` configuration, it must
# orchestrate VMs in GCP directly.  This means users need to have
# pre-authorization (access) to manipulate google-cloud resources.  Additionally,
# there are no guarantees it will remain in-sync with other automation-related
# scripts.  Therefore it may not always function for everybody in every
# future scenario without updates/modifications/tweaks.
#
# When successful, you will end up connected to a GCP VM with with a clone of
# the upstream podman repository 'master' branch, using a remote named 'origin'.
# If you want to customize this behavior, you will want to use a "hook" script.
# Please use this example carefully, since git setups vary by person, you
# will probably need to make local edits.
#
# https://gist.github.com/cevich/626a0790c0b476d5cd2a5a76fbdae0a1

set -e

RED="\e[1;31m"
YEL="\e[1;32m"
NOR="\e[0m"
USAGE_WARNING="
${YEL}WARNING: This will not work without podman,${NOR}
         ${YEL}and prior authorization to use the libpod GCP project.${NOR}
"
# These values come from .cirrus.yml gce_instance clause
ZONE="${ZONE:-us-central1-a}"
CPUS="2"
MEMORY="4Gb"
DISK="200"
PROJECT="libpod-218412"
GOSRC="/var/tmp/go/src/github.com/containers/podman"
GIT_REPO="https://github.com/containers/podman.git"

# Container image with necessary runtime elements
GCLOUD_IMAGE="${GCLOUD_IMAGE:-docker.io/google/cloud-sdk:alpine}"
GCLOUD_CFGDIR=".config/gcloud"

SCRIPT_FILENAME=$(basename ${BASH_SOURCE[0]})
HOOK_FILENAME="hook_${SCRIPT_FILENAME}"

# Shared tmp directory between container and us
TMPDIR=$(mktemp -d --tmpdir ${SCRIPT_FILENAME}_tmpdir_XXXXXX)

show_usage() {
    echo -e "\n${RED}ERROR: $1${NOR}"
    echo -e "${YEL}Usage: $SCRIPT_FILENAME <image_name>${NOR}"
    echo ""
    if [[ -r ".cirrus.yml" ]]
    then
        echo -e "${YEL}Some possible image_name values (from .cirrus.yml):${NOR}"
        image_hints
        echo ""
        echo -e "${YEL}Optional:${NOR} If a $HOME/$GCLOUD_CFGDIR/$HOOK_FILENAME executable exists during"
        echo "VM creation, it will be executed remotely after cloning"
        echo "$GIT_REPO. The"
        echo "current local working branch name and commit ID, will be provided as"
        echo "it's arguments."
    fi
    exit 1
}

LIBPODROOT=$(realpath "$(dirname ${BASH_SOURCE[0]})/../")
# else: Assume $PWD is the root of the libpod repository
[[ "$LIBPODROOT" != "/" ]] || \
    show_usage "Must execute script from within clone of containers/podman repo."

[[ "$UID" -ne 0 ]] || \
    show_usage "Must execute script as a regular (non-root) user."

[[ "${LIBPODROOT#$HOME}" != "$LIBPODROOT" ]] || \
    show_usage "Clone of containers/podman must be a subdirectory of \$HOME ($HOME)"

# Disable SELinux labeling to allow read-only mounting of repository files
PGCLOUD="podman run -it --rm --security-opt label=disable -v $TMPDIR:$TMPDIR -v $HOME/.config/gcloud:/root/.config/gcloud -v $HOME/.config/gcloud/ssh:/root/.ssh -v $LIBPODROOT:$LIBPODROOT:ro $GCLOUD_IMAGE gcloud --configuration=libpod --project=$PROJECT"
SCP_CMD="$PGCLOUD compute scp"

showrun() {
    echo '+ '$(printf " %q" "$@") > /dev/stderr
    echo ""
    "$@"
}

cleanup() {
    RET=$?
    set +e
    wait

    # set GCLOUD_DEBUG to leave tmpdir behind for postmortem
    # shellcheck disable=SC2154
    test -z "$GCLOUD_DEBUG" && rm -rf $TMPDIR

    # Not always called from an exit handler, but should always exit when called
    exit $RET
}
trap cleanup EXIT

delvm() {
    echo -e "\n"
    echo -e "\n${YEL}Offering to Delete $VMNAME${NOR}"
    echo -e "${RED}(Deletion might take a minute or two)${NOR}"
    echo -e "${YEL}Note: It's safe to answer N, then re-run script again later.${NOR}"
    showrun $CLEANUP_CMD  # prompts for Yes/No
    cleanup
}

get_env_vars() {
    # Deal with both YAML and embedded shell-like substitutions in values
    # if substitution fails, fall back to printing naked env. var as-is.
    python3 -c '
import sys,yaml,re
env=yaml.load(open(".cirrus.yml"), Loader=yaml.SafeLoader)["env"]
dollar_env_var=re.compile(r"\$(\w+)")
dollarcurly_env_var=re.compile(r"\$\{(\w+)\}")
class ReIterKey(dict):
    def __missing__(self, key):
        # Cirrus-CI provides some runtime-only env. vars.  Avoid
        # breaking this hack-script if/when any are present in YAML
        return "${0}".format(key)
rep=r"{\1}"  # Convert env vars markup to -> str.format_map(re_iter_key) markup
out=ReIterKey()
for k,v in env.items():
    if "ENCRYPTED" not in str(v) and bool(v):
        out[k]=dollar_env_var.sub(rep, dollarcurly_env_var.sub(rep, str(v)))
for k,v in out.items():
    sys.stdout.write("{0}=\"{1}\"\n".format(k, str(v).format_map(out)))
    '
}

image_hints() {
    get_env_vars | fgrep '_CACHE_IMAGE_NAME' | awk -F "=" '{print $2}'
}

unset VM_IMAGE_NAME
unset VMNAME
unset CREATE_CMD
unset SSH_CMD
unset CLEANUP_CMD
declare -xa ENVS
parse_args(){
    local arg
    echo -e "$USAGE_WARNING"

    if [[ "$USER" =~ "root" ]]
    then
        show_usage "This script must be run as a regular user."
    fi

    [[ "$#" -eq 1 ]] || \
        show_usage "Must specify a VM Image name to use, and the test flavor."

    VM_IMAGE_NAME="$1"

    # Word-splitting is desirable in this case.
    # Values are used literally (with '=') as args to future `env` command.
    # get_env_vars() will take care of properly quoting it's output.
    # shellcheck disable=SC2207,SC2191
    ENVS=(
        $(get_env_vars)
        VM_IMAGE_NAME="$VM_IMAGE_NAME"
        UPSTREAM_REMOTE="upstream"
    )

    VMNAME="${VMNAME:-${USER}-${VM_IMAGE_NAME}}"

    CREATE_CMD="$PGCLOUD compute instances create --zone=$ZONE --image=${VM_IMAGE_NAME} --custom-cpu=$CPUS --custom-memory=$MEMORY --boot-disk-size=$DISK --labels=in-use-by=$USER $VMNAME"

    SSH_CMD="$PGCLOUD compute ssh root@$VMNAME"

    CLEANUP_CMD="$PGCLOUD compute instances delete --zone $ZONE --delete-disks=all $VMNAME"
}

# Returns true if user has run an 'init' and has a valid token for
# the specific project-id and named-configuration arguments in $PGCLOUD.
function has_valid_credentials() {
    if $PGCLOUD info |& grep -Eq 'Account:.*None'; then
        return 1
    fi

    # It's possible for 'gcloud info' to list expired credentials,
    # e.g. 'ERROR:  ... invalid grant: Bad Request'
    if $PGCLOUD auth print-access-token |& grep -q 'ERROR'; then
        return 1
    fi

    return 0
}

##### main

[[ "${LIBPODROOT%%${LIBPODROOT##$HOME}}" == "$HOME" ]] || \
    show_usage "Repo clone must be sub-dir of $HOME"

cd "$LIBPODROOT"

parse_args "$@"
mkdir -p $TMPDIR/.ssh
mkdir -p {$HOME,$TMPDIR}/.config/gcloud/ssh
chmod 700 {$HOME,$TMPDIR}/.config/gcloud/ssh $TMPDIR/.ssh

echo -e "\n${YEL}Pulling gcloud image...${NOR}"
podman pull $GCLOUD_IMAGE

if ! has_valid_credentials
then
    echo -e "\n${YEL}WARNING: Can't find gcloud configuration for libpod, running init.${NOR}"
    echo -e "         ${RED}Please choose \"#1: Re-initialize\" and \"login\" if asked.${NOR}"
    showrun $PGCLOUD init --project=$PROJECT --console-only --skip-diagnostics

    # Verify it worked (account name == someone@example.com)
    $PGCLOUD info > $TMPDIR/gcloud-info-after-init
    if egrep -q "Account:.*None" $TMPDIR/gcloud-info-after-init
    then
        echo -e "${RED}ERROR: Could not initialize libpod configuration in gcloud.${NOR}"
        exit 5
    fi

    # If this is the only config, make it the default to avoid
    # persistent warnings from gcloud about there being no default.
    [[ -r "$HOME/.config/gcloud/configurations/config_default" ]] || \
       ln "$HOME/.config/gcloud/configurations/config_libpod" \
          "$HOME/.config/gcloud/configurations/config_default"
fi

trap delvm EXIT # Allow deleting VM if CTRL-C during create
echo -e "\n${YEL}Trying to creating a VM named $VMNAME${NOR}\n${YEL}in GCE region/zone $ZONE${NOR}"
echo -e "For faster terminal access, export ZONE='<something-closer>'"
echo -e 'Zone-list at: https://cloud.google.com/compute/docs/regions-zones/\n'
if showrun $CREATE_CMD; then  # Freshly created VM needs initial setup

    echo -e "\n${YEL}Waiting up to 30s for ssh port to open${NOR}"
    ATTEMPTS=10
    trap "exit 1" INT
    while ((ATTEMPTS)) && ! $SSH_CMD --command "true"; do
        let "ATTEMPTS--"
        echo -e "${RED}Nope, not yet.${NOR}"
        sleep 3s
    done
    trap - INT
    if ! ((ATTEMPTS)); then
        echo -e "\n${RED}Failed${NOR}"
        exit 7
    fi
    echo -e "${YEL}Got it.  Cloning upstream repository as a starting point.${NOR}"

    showrun $SSH_CMD -- "mkdir -p $GOSRC"
    showrun $SSH_CMD -- "git clone --progress $GIT_REPO $GOSRC"

    if [[ -x "$HOME/$GCLOUD_CFGDIR/$HOOK_FILENAME" ]]; then
        echo -e "\n${YEL}Copying hook to VM and executing (ignoring errors).${NOR}"
        $PGCLOUD compute scp "/root/$GCLOUD_CFGDIR/$HOOK_FILENAME" root@$VMNAME:.
        if ! showrun $SSH_CMD -- "cd $GOSRC && bash /root/$HOOK_FILENAME $(git branch --show-current) $(git rev-parse HEAD)"; then
            echo "-e ${RED}Hook exited: $?${NOR}"
        fi
    fi
fi

echo -e "\n${YEL}Generating connection script for $VMNAME.${NOR}"
echo -e "Note: Script can be re-used in another terminal if needed."
echo -e "${RED}(option to delete VM presented upon exiting).${NOR}"
# TODO: This is fairly fragile, specifically the quoting for the remote command.
echo '#!/bin/bash' > $TMPDIR/ssh
echo "$SSH_CMD -- -t 'cd $GOSRC && exec env ${ENVS[*]} bash -il'" >> $TMPDIR/ssh
chmod +x $TMPDIR/ssh

showrun $TMPDIR/ssh
