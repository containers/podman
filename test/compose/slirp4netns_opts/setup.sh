# -*- bash -*-

# create tempfile to store nc output
OUTFILE=$(mktemp)
# listen on a port, the container will try to connect to it
nc -l 5001 > $OUTFILE &

nc_pid=$!
