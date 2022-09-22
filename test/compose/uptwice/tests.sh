# -*- bash -*-

CR=$'\r'
NL=$'\n'

cp docker-compose.yml docker-compose.yml.bak
sed -i -e 's/10001/10002/' docker-compose.yml
output=$(docker-compose up -d 2>&1)

# Horrible output check here but we really want to make sure that there are
# no unexpected warning/errors and the normal messages are send on stderr as
# well so we cannot check for an empty stderr.
expected="Recreating uptwice_app_1 ... ${CR}${NL}Recreating uptwice_app_1 ... done$CR"
if [ "$TEST_FLAVOR" = "compose_v2" ]; then
    expected="Container uptwice-app-1  Recreate${NL}Container uptwice-app-1  Recreated${NL}Container uptwice-app-1  Starting${NL}Container uptwice-app-1  Started"
fi
is "$output" "$expected" "no error output in compose up (#15580)"
