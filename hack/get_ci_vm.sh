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
ZONE="${ZONE:-us-central1-a}"
CPUS="2"
MEMORY="4Gb"
DISK="200"
PROJECT="libpod-218412"
GOSRC="/var/tmp/go/src/github.com/containers/libpod"
GCLOUD_IMAGE=${GCLOUD_IMAGE:-quay.io/cevich/gcloud_centos:latest}
GCLOUD_SUDO=${GCLOUD_SUDO-sudo}

# Shared tmp directory between container and us
TMPDIR=$(mktemp -d --tmpdir $(basename $0)_tmpdir_XXXXXX)

LIBPODROOT=$(realpath "$(dirname $0)/../")
# else: Assume $PWD is the root of the libpod repository
[[ "$LIBPODROOT" != "/" ]] || LIBPODROOT=$PWD

# Command shortcuts save some typing (asumes $LIBPODROOT is subdir of $HOME)
PGCLOUD="$GCLOUD_SUDO podman run -it --rm -e AS_ID=$UID -e AS_USER=$USER --security-opt label=disable -v $TMPDIR:$HOME -v $HOME/.config/gcloud:$HOME/.config/gcloud -v $HOME/.config/gcloud/ssh:$HOME/.ssh -v $LIBPODROOT:$LIBPODROOT $GCLOUD_IMAGE --configuration=libpod --project=$PROJECT"
SCP_CMD="$PGCLOUD compute scp"


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
    RET=$?
    set +e
    wait

    # set GCLOUD_DEBUG to leave tmpdir behind for postmortem
    test -z "$GCLOUD_DEBUG" && rm -rf $TMPDIR

    # Not always called from an exit handler, but should always exit when called
    exit $RET
}
trap cleanup EXIT

delvm() {
    echo -e "\n"
    echo -e "\n${YEL}Offering to Delete $VMNAME ${RED}(Might take a minute or two)${NOR}"
    echo -e "\n${YEL}Note: It's safe to answer N, then re-run script again later.${NOR}"
    showrun $CLEANUP_CMD  # prompts for Yes/No
    cleanup
}

show_usage() {
    echo -e "\n${RED}ERROR: $1${NOR}"
    echo -e "${YEL}Usage: $(basename $0) [-m <SPECIALMODE>] [-u <ROOTLESS_USER> ] <image_name>${NOR}"
    echo "Use -m <SPECIALMODE> with a supported value documented in contrib/cirrus/README.md."
    echo "With '-m rootless' must also specify -u <ROOTLESS_USER> with name of user to create & use"
    echo ""
    if [[ -r ".cirrus.yml" ]]
    then
        echo -e "${YEL}Some possible image_name values (from .cirrus.yml):${NOR}"
        image_hints
        echo ""
    fi
    exit 1
}

get_env_vars() {
    # Deal with both YAML and embedded shell-like substitutions in values
    # if substitution fails, fall back to printing naked env. var as-is.
    python3 -c '
import yaml,re
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
    v=str(v)
    if "ENCRYPTED" not in v:
        out[k]=dollar_env_var.sub(rep, dollarcurly_env_var.sub(rep, v))
for k,v in out.items():
    print("{0}=\"{1}\"".format(k, v.format_map(out)))
    '
}

image_hints() {
    get_env_vars | fgrep '_CACHE_IMAGE_NAME' | awk -F "=" '{print $2}'
}


parse_args(){
    echo -e "$USAGE_WARNING"

    if [[ "$USER" =~ "root" ]]
    then
        show_usage "This script must be run as a regular user."
    fi

    ENVS="$(get_env_vars)"
    [[ "$#" -ge "1" ]] || \
        show_usage "Must specify at least one command-line parameter."

    IMAGE_NAME=""
    ROOTLESS_USER=""
    SPECIALMODE="none"
    for arg
    do
        if [[ "$SPECIALMODE" == "GRABNEXT" ]] && [[ "${arg:0:1}" != "-" ]]
        then
            SPECIALMODE="$arg"
            echo -e "${YEL}Using \$SPECIALMODE=$SPECIALMODE.${NOR}"
            continue
        elif [[ "$ROOTLESS_USER" == "GRABNEXT" ]] && [[ "${arg:0:1}" != "-" ]]
        then
            ROOTLESS_USER="$arg"
            echo -e "${YEL}Using \$ROOTLESS_USER=$ROOTLESS_USER.${NOR}"
            continue
        fi
        case "$arg" in
            -m)
                SPECIALMODE="GRABNEXT"
                ;;
            -u)
                ROOTLESS_USER="GRABNEXT"
                ;;
            *)
                [[ "${arg:0:1}" != "-" ]] || \
                    show_usage "Unknown command-line option '$arg'."
                [[ -z "$IMAGE_NAME" ]] || \
                    show_usage "Must specify exactly one image name, got '$IMAGE_NAME' and '$arg'."
                IMAGE_NAME="$arg"
                ;;
        esac
    done

    if [[ "$SPECIALMODE" == "GRABNEXT" ]]
    then
        show_usage "Must specify argument to -m option."
    fi

    if [[ "$ROOTLESS_USER" == "GRABNEXT" ]]
    then
        show_usage "Must specify argument to -u option."
    fi

    if [[ -z "$IMAGE_NAME" ]]
    then
        show_usage "No image-name specified."
    fi

    if [[ "$SPECIALMODE" == "rootless" ]] && [[ -z "$ROOTLESS_USER" ]]
    then
        show_usage "With '-m rootless' must also pass -u <username> of rootless user."
    fi

    if echo "$IMAGE_NAME" | grep -q "image-builder-image"
    then
        echo -e "Creating an image-builder VM, I hope you know what you're doing.\n"
        IBI_ARGS="--scopes=compute-rw,storage-rw,userinfo-email"
        SSHUSER="centos"
    else
        unset IBI_ARGS
        SSHUSER="root"
    fi

    ENVS="$ENVS SPECIALMODE=\"$SPECIALMODE\""

    [[ -z "$ROOTLESS_USER" ]] || \
        ENVS="$ENVS ROOTLESS_USER=$ROOTLESS_USER"

    SETUP_CMD="env $ENVS ADD_SECOND_PARTITIO=True $GOSRC/contrib/cirrus/setup_environment.sh"
    VMNAME="${VMNAME:-${USER}-${IMAGE_NAME}}"

    CREATE_CMD="$PGCLOUD compute instances create --zone=$ZONE --image=${IMAGE_NAME} --custom-cpu=$CPUS --custom-memory=$MEMORY --boot-disk-size=$DISK --labels=in-use-by=$USER $IBI_ARGS $VMNAME"

    SSH_CMD="$PGCLOUD compute ssh $SSHUSER@$VMNAME"

    CLEANUP_CMD="$PGCLOUD compute instances delete --zone $ZONE --delete-disks=all $VMNAME"
}

##### main

[[ "${LIBPODROOT%%${LIBPODROOT##$HOME}}" == "$HOME" ]] || \
    show_usage "Repo clone must be sub-dir of $HOME"

cd "$LIBPODROOT"

parse_args "$@"

# Ensure mount-points and data directories exist on host as $USER.  Also prevents
# permission-denied errors during cleanup() b/c `sudo podman` created mount-points
# owned by root.
mkdir -p $TMPDIR/${LIBPODROOT##$HOME}
mkdir -p $TMPDIR/.ssh
mkdir -p {$HOME,$TMPDIR}/.config/gcloud/ssh
chmod 700 {$HOME,$TMPDIR}/.config/gcloud/ssh $TMPDIR/.ssh

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

# Couldn't make rsync work with gcloud's ssh wrapper because ssh-keys generated on the fly
TARBALL=$VMNAME.tar.bz2
echo -e "\n${YEL}Packing up local repository into a tarball.${NOR}"
showrun --background tar cjf $TMPDIR/$TARBALL --warning=no-file-changed --exclude-vcs-ignores -C $LIBPODROOT .

trap delvm INT  # Allow deleting VM if CTRL-C during create
# This fails if VM already exists: permit this usage to re-init
echo -e "\n${YEL}Trying to creating a VM named $VMNAME${NOR}\n${YEL}in GCE region/zone $ZONE${NOR}"
echo -e "For faster access, export ZONE='something-closer-<any letter>'"
echo 'List of regions and zones: https://cloud.google.com/compute/docs/regions-zones/'
echo -e "${RED}(might take a minute/two.  Errors ignored).${NOR}"
showrun $CREATE_CMD || true # allow re-running commands below when "delete: N"

# Any subsequent failure should prompt for VM deletion
trap - INT
trap delvm EXIT

echo -e "\n${YEL}Waiting up to 30s for ssh port to open${NOR}"
trap 'COUNT=9999' INT
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

echo -e "\n${YEL}Removing and re-creating $GOSRC on $VMNAME.${NOR}"
showrun $SSH_CMD --command "rm -rf $GOSRC"
showrun $SSH_CMD --command "mkdir -p $GOSRC"

echo -e "\n${YEL}Transferring tarball to $VMNAME.${NOR}"
wait
showrun $SCP_CMD $HOME/$TARBALL $SSHUSER@$VMNAME:/tmp/$TARBALL

echo -e "\n${YEL}Unpacking tarball into $GOSRC on $VMNAME.${NOR}"
showrun $SSH_CMD --command "tar xjf /tmp/$TARBALL -C $GOSRC"

echo -e "\n${YEL}Removing tarball on $VMNAME.${NOR}"
showrun $SSH_CMD --command "rm -f /tmp/$TARBALL"

echo -e "\n${YEL}Executing environment setup${NOR}"
showrun $SSH_CMD --command "$SETUP_CMD"

VMIP=$($PGCLOUD compute instances describe $VMNAME --format='get(networkInterfaces[0].accessConfigs[0].natIP)')

echo -e "\n${YEL}Connecting to $VMNAME${NOR}\nPublic IP Address: $VMIP\n${RED}(option to delete VM upon logout).${NOR}\n"
if [[ -n "$ROOTLESS_USER" ]]
then
    echo "Re-chowning source files after transfer"
    showrun $SSH_CMD --command "chown -R $ROOTLESS_USER $GOSRC"
    echo "Connecting as user $ROOTLESS_USER"
    SSH_CMD="$PGCLOUD compute ssh $ROOTLESS_USER@$VMNAME"
fi
showrun $SSH_CMD -- -t "cd $GOSRC && exec env $ENVS bash -il"
