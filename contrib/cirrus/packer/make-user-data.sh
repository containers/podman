#!/bin/bash

# This script is utilized by Makefile, it's not intended to be run by humans

cat <<EOF > user-data
#cloud-config
timezone: US/Eastern
growpart:
    mode: auto
disable_root: false
ssh_pwauth: True
ssh_import_id: [root]
ssh_authorized_keys:
    - $(cat cidata.ssh.pub)
users:
   - name: root
     primary-group: root
     homedir: /root
     system: true
EOF
