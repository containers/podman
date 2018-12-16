#!/bin/bash

set -e

cd $(dirname $0)/../

VMNAME="${USER}-twidling-$1"
# TODO: Many/most of these values should come from .cirrus.yml
ZONE="us-central1-a"
CPUS="2"
MEMORY="4Gb"
DISK="200"
PROJECT="libpod-218412"
GOSRC="/var/tmp/go/src/github.com/containers/libpod"

PGCLOUD="sudo podman run -it --rm -e AS_ID=$UID -e AS_USER=$USER -v /home/$USER:$HOME:z quay.io/cevich/gcloud_centos:latest"
CREATE_CMD="$PGCLOUD compute instances create --zone=$ZONE --image=$1 --custom-cpu=$CPUS --custom-memory=$MEMORY --boot-disk-size=$DISK --labels=in-use-by=$USER $VMNAME"
SSH_CMD="ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -o CheckHostIP=no -F /dev/null"
CLEANUP_CMD="$PGCLOUD compute instances delete --zone $ZONE --delete-disks=all $VMNAME"

# COLOR!
RED="\e[1;36;41m"
YEL="\e[1;33;44m"
NOR="\e[0m"

if [[ -z "$1" ]]
then
    echo -e "\n${RED}Error: No image-name specified.  Some possible values (from .cirrus.yml).${NOR}"
    egrep 'image_name' ".cirrus.yml" | grep -v '#' | cut -d: -f 2 | tr -d [:blank:]
    exit 1
fi

echo -e "\n${YEL}WARNING: This will not work without local sudo access to run podman,${NOR}"
echo -e "         ${YEL}and prior authorization to use the libpod GCP project. Also,${NOR}"
echo -e "         ${YEL}possession of the proper ssh private key is required.${NOR}"

if [[ "$USER" =~ "root" ]]
then
    echo -e "\n${RED}ERROR: This script must be run as a regular user${NOR}"
    exit 2
fi

if [[ ! -r "$HOME/.config/gcloud/active_config" ]]
then
    echo -e "\n${RED}ERROR: Can't find gcloud configuration, attempting to run init.${NOR}"
    $PGCLOUD init --project=$PROJECT
fi

cleanup() {
    echo -e "\n${YEL}Deleting $VMNAME ${RED}(Might take a minute or two)${NOR}
+ $CLEANUP_CMD
"
    $CLEANUP_CMD  # prompts for Yes/No
}

trap cleanup EXIT

echo -e "\n${YEL}Trying to creating a VM named $VMNAME (not fatal if already exists).${NOR}"
echo "+ $CREATE_CMD"
$CREATE_CMD || true # allow re-running commands below when "delete: N"

echo -e "\n${YEL}Attempting to retrieve IP address of existing ${VMNAME}${NOR}."
IP=`$PGCLOUD compute instances list --filter=name=$VMNAME --limit=1 '--format=csv(networkInterfaces.accessConfigs.natIP)' | tr --complement --delete .[:digit:]`

echo -e "\n${YEL}Creating $GOSRC directory.${NOR}"
SSH_MKDIR="$SSH_CMD root@$IP mkdir -vp $GOSRC"
echo "+ $SSH_MKDIR"
$SSH_MKDIR

echo -e "\n${YEL}Synchronizing local repository to $IP:${GOSRC}${NOR} ."
export RSYNC_RSH="$SSH_CMD"
RSYNC_CMD="rsync --quiet --recursive --update --links --safe-links --perms --sparse $PWD/ root@$IP:$GOSRC/"
echo "+ export RSYNC_RSH=\"$SSH_CMD\""
echo "+ $RSYNC_CMD"
$RSYNC_CMD

echo -e "\n${YEL}Executing environment setup${NOR}"
ENV_CMD="$SSH_CMD root@$IP env CI=true $GOSRC/contrib/cirrus/setup_environment.sh"
echo "+ $ENV_CMD"
$SSH_CMD root@$IP $GOSRC/contrib/cirrus/setup_environment.sh

echo -e "\n${YEL}Connecting to $VMNAME ${RED}(option to delete VM upon logout).${NOR}"
SSH_CMD="$SSH_CMD -t root@$IP"
echo "+ $SSH_CMD"
$SSH_CMD "cd $GOSRC ; bash -il"
