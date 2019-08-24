#!/bin/bash
if pkg-config libbcc 2> /dev/null; then
    echo oci_trace_hook
fi