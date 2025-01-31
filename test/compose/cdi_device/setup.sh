if is_rootless; then
    reason=" - can't write to /etc/cdi"
    _show_ok skip "$testname # skip$reason"
    exit 0
fi

mkdir -p /etc/cdi
mount -t tmpfs tmpfs /etc/cdi
cp device.json /etc/cdi
