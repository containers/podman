#!/usr/bin/env bash
if pkg-config --exists libsystemd; then
    echo systemd
fi
