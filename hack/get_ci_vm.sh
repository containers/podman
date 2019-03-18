#!/bin/bash

set -e

RED="\e[1;36;41m"
YEL="\e[1;33;44m"
NOR="\e[0m"
USAGE_WARNING="
${YEL}WARNING: This will not work without local sudo access to run podman,${NOR}
         ${YEL}and prior authorization to use the libpod GCP project. Also,${NOR}
         ${YEL}possession of the proper ssh private key is required.${NOR}
"
# TODO: Many/most of these values should come from .cirrus.yml
ZONE="us-central1-a"
CPUS="2"
MEMORY="4Gb"
DISK="200"
PROJECT="libpod-218412"
GOSRC="/var/tmp/go/src/github.com/containers/libpod"
GCLOUD_IMAGE=${GCLOUD_IMAGE:-quay.io/cevich/gcloud_centos:latest}
GCLOUD_SUDO=${GCLOUD_SUDO-sudo}
ROOTLESS_USER="madcowdog"

# Shared tmp directory between container and us
TMPDIR=$(mktemp -d --tmpdir $(basename $0)_tmpdir_XXXXXX)

# Command shortcuts save some typing
PGCLOUD="$GCLOUD_SUDO podman run -it --rm -e AS_ID=$UID -e AS_USER=$USER --security-opt label=disable -v /home/$USER:$HOME -v $TMPDIR:/tmp $GCLOUD_IMAGE --configuration=libpod --project=$PROJECT"
SCP_CMD="$PGCLOUD compute scp"

LIBPODROOT=$(realpath "$(dirname $0)/../")
# else: Assume $PWD is the root of the libpod repository
[[ "$LIBPODROOT" != "/" ]] || LIBPODROOT=$PWD

showrun() {
    if [[ "$1" == "--background" ]]
    then
        shift
        # Properly escape any nested spaces, so command can be copy-pasted
        echo '+ '$(printf " %q" "$@")' &' > /dev/stderr
        "$@" &
        echo -e "${RED}<backgrounded>${NOR}"
    else
        echo '+ '$(printf " %q" "$@") > /dev/stderr
        "$@"
    fi
}

cleanup() {
    set +e
    wait

    # set GCLOUD_DEBUG to leave tmpdir behind for postmortem
    test -z "$GCLOUD_DEBUG" && rm -rf $TMPDIR
}
trap cleanup EXIT

delvm() {
    echo -e "\n"
    echo -e "\n${YEL}Offering to Delete $VMNAME ${RED}(Might take a minute or two)${NOR}"
    showrun $CLEANUP_CMD  # prompts for Yes/No
    cleanup
}

image_hints() {
    egrep '[[:space:]]+[[:alnum:]].+_CACHE_IMAGE_NAME:[[:space:]+"[[:print:]]+"' \
        "$LIBPODROOT/.cirrus.yml" | cut -d: -f 2 | tr -d '"[:blank:]' | \
        grep -v 'notready' | grep -v 'image-builder' | sort -u
}

show_usage() {
    echo -e "\n${RED}ERROR: $1${NOR}"
    echo -e "${YEL}Usage: $(basename $0) [-s | -p | -r] <image_name>${NOR}"
    echo "Use -s / -p to select source or package based dependencies"
    echo -e "Use -r to setup and run tests as a regular user.\n"
    if [[ -r ".cirrus.yml" ]]
    then
        echo -e "${YEL}Some possible image_name values (from .cirrus.yml):${NOR}"
        image_hints
        echo ""
    fi
    exit 1
}

get_env_vars() {
    python -c '
import yaml
env=yaml.load(open(".cirrus.yml"))["env"]
keys=[k for k in env if "ENCRYPTED" not in str(env[k])]
for k,v in env.items():
    v=str(v)
    if "ENCRYPTED" not in v:
        print "{0}=\"{1}\"".format(k, v),
    '
}

parse_args(){
    if [[ -z "$1" ]]
    then
        show_usage "Must specify at least one command-line parameter."
    elif [[ "$1" == "-p" ]]
    then
        DEPS="PACKAGE_DEPS=true SOURCE_DEPS=false"
        IMAGE_NAME="$2"

    elif [[ "$1" == "-s" ]]
    then
        DEPS="PACKAGE_DEPS=false SOURCE_DEPS=true"
        IMAGE_NAME="$2"
    elif [[ "$1" == "-r" ]]
    then
        DEPS="ROOTLESS_USER=$ROOTLESS_USER ROOTLESS_UID=3210 ROOTLESS_GID=3210"
        IMAGE_NAME="$2"
    else  # no -s or -p
        DEPS="$(get_env_vars)"
        IMAGE_NAME="$1"
    fi

    if [[ -z "$IMAGE_NAME" ]]
    then
        show_usage "No image-name specified."
    fi

    if [[ "$USER" =~ "root" ]]
    then
        show_usage "This script must be run as a regular user."
    fi

    echo -e "$USAGE_WARNING"

    SETUP_CMD="env $DEPS $GOSRC/contrib/cirrus/setup_environment.sh"
    VMNAME="${VMNAME:-${USER}-${IMAGE_NAME}}"
    CREATE_CMD="$PGCLOUD compute instances create --zone=$ZONE --image=${IMAGE_NAME} --custom-cpu=$CPUS --custom-memory=$MEMORY --boot-disk-size=$DISK --labels=in-use-by=$USER $VMNAME"
    SSH_CMD="$PGCLOUD compute ssh root@$VMNAME"
    CLEANUP_CMD="$PGCLOUD compute instances delete --zone $ZONE --delete-disks=all $VMNAME"
}

##### main

parse_args $@

cd $LIBPODROOT

# Attempt to determine if named 'libpod' gcloud configuration exists
showrun $PGCLOUD info > $TMPDIR/gcloud-info
if egrep -q "Account:.*None" $TMPDIR/gcloud-info
then
    echo -e "\n${YEL}WARNING: Can't find gcloud configuration for libpod, running init.${NOR}"
    echo -e "         ${RED}Please choose "#1: Re-initialize" and "login" if asked.${NOR}"
    showrun $PGCLOUD init --project=$PROJECT --console-only --skip-diagnostics

    # Verify it worked (account name == someone@example.com)
    $PGCLOUD info > $TMPDIR/gcloud-info-after-init
    if egrep -q "Account:.*None" $TMPDIR/gcloud-info-after-init
    then
        echo -e "${RED}ERROR: Could not initialize libpod configuration in gcloud.${NOR}"
        exit 5
    fi

    # If this is the only config, make it the default to avoid persistent warnings from gcloud
    [[ -r "$HOME/.config/gcloud/configurations/config_default" ]] || \
        ln "$HOME/.config/gcloud/configurations/config_libpod" \
           "$HOME/.config/gcloud/configurations/config_default"
fi

# Couldn't make rsync work with gcloud's ssh wrapper :(
TARBALL_BASENAME=$VMNAME.tar.bz2
TARBALL=/tmp/$TARBALL_BASENAME
echo -e "\n${YEL}Packing up repository into a tarball $VMNAME.${NOR}"
showrun --background tar cjf $TMPDIR/$TARBALL_BASENAME --warning=no-file-changed -C $LIBPODROOT .

trap delvm INT  # Allow deleting VM if CTRL-C during create
# This fails if VM already exists: permit this usage to re-init
echo -e "\n${YEL}Trying to creating a VM named $VMNAME ${RED}(might take a minute/two.  Errors ignored).${NOR}"
showrun $CREATE_CMD || true # allow re-running commands below when "delete: N"

# Any subsequent failure should prompt for VM deletion
trap delvm EXIT

echo -e "\n${YEL}Waiting up to 30s for ssh port to open${NOR}"
ATTEMPTS=10
for (( COUNT=1 ; COUNT <= $ATTEMPTS ; COUNT++ ))
do
    if $SSH_CMD --command "true"; then break; else sleep 3s; fi
done
if (( COUNT > $ATTEMPTS ))
then
    echo -e "\n${RED}Failed${NOR}"
    exit 7
fi
echo -e "${YEL}Got it${NOR}"

if $SSH_CMD --command "test -r /root/.bash_profile_original"
then
    echo -e "\n${YEL}Resetting environment configuration${NOR}"
    showrun $SSH_CMD --command "cp /root/.bash_profile_original /root/.bash_profile"
fi

echo -e "\n${YEL}Removing and re-creating $GOSRC on $VMNAME.${NOR}"
showrun $SSH_CMD --command "rm -rf $GOSRC"
showrun $SSH_CMD --command "mkdir -p $GOSRC"

echo -e "\n${YEL}Transfering tarball to $VMNAME.${NOR}"
wait
showrun $SCP_CMD $TARBALL root@$VMNAME:$TARBALL

echo -e "\n${YEL}Unpacking tarball into $GOSRC on $VMNAME.${NOR}"
showrun $SSH_CMD --command "tar xjf $TARBALL -C $GOSRC"

echo -e "\n${YEL}Removing tarball on $VMNAME.${NOR}"
showrun $SSH_CMD --command "rm -f $TARBALL"

echo -e "\n${YEL}Executing environment setup${NOR}"
[[ "$1" == "-p" ]] && echo -e "${RED}Using package-based dependencies.${NOR}"
[[ "$1" == "-s" ]] && echo -e "${RED}Using source-based dependencies.${NOR}"
showrun $SSH_CMD --command "$SETUP_CMD"

echo -e "\n${YEL}Connecting to $VMNAME ${RED}(option to delete VM upon logout).${NOR}\n"
if [[ "$1" == "-r" ]]
then
    SSH_CMD="$PGCLOUD compute ssh $ROOTLESS_USER@$VMNAME"
fi
showrun $SSH_CMD -- -t "cd $GOSRC && exec env $DEPS bash -il"
