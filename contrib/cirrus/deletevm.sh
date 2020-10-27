#!/bin/bash

set -eo pipefail

# This temporary/debugging script simply removes any deletion-protection
# from the current VM.  It's intended to be executed from an 'always' script.
# We assume that an agent-stopped-reesponding event will NOT execute an
# always script, but simple test failures will.

# shellcheck source=contrib/cirrus/lib.sh
source $(dirname $0)/lib.sh

# Assumed running on a GCP VM w/ activated & configured account
warn "Enabling automatic VM deletion by Cirrus-CI"
$GCLOUD_CNTNR compute instances update $HOSTNAME --no-deletion-protection --clear-labels --update-labels=cciasr=1
