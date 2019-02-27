#!/bin/sh
./bin/podman --help | sed -n -Ee '0,/Available Commands/d' -e '/^Flags/q;p' | sed '/^$/d' | awk '{ print $1 }' > /tmp/podman.cmd
man ./docs/podman.1 | sed -n -e '0,/COMMANDS/d' -e '/^FILES/q;p' | grep podman | cut -f2 -d- | cut -f1 -d\( > /tmp/podman.man
echo diff -B -u /tmp/podman.cmd /tmp/podman.man
diff -B -u /tmp/podman.cmd /tmp/podman.man

./bin/podman image --help | sed -n -e '0,/Available Commands/d' -e '/^Flags/q;p' | sed '/^$/d' | awk '{ print $1 }' > /tmp/podman-image.cmd
man ./docs/podman-image.1 | sed -n -Ee '0,/COMMANDS/d'  -e 's/^[[:space:]]*//' -e '/^SEE ALSO/q;p' | grep podman | cut -f1 -d' ' | sed 's/^.//' > /tmp/podman-image.man
echo diff -B -u /tmp/podman-image.cmd /tmp/podman-image.man
diff -B -u /tmp/podman-image.cmd /tmp/podman-image.man

./bin/podman container --help | sed -n -e '0,/Available Commands/d' -e '/^Flags/q;p' | sed '/^$/d' | awk '{ print $1 }' > /tmp/podman-container.cmd
man docs/podman-container.1 | sed -n -Ee '0,/COMMANDS/d'  -e 's/^[[:space:]]*//' -e '/^SEE ALSO/q;p' | grep podman | cut -f1 -d' ' | sed 's/^.//' > /tmp/podman-container.man
echo diff -B -u /tmp/podman-container.cmd /tmp/podman-container.man
diff -B -u /tmp/podman-container.cmd /tmp/podman-container.man

./bin/podman system --help | sed -n -e '0,/Available Commands/d' -e '/^Flags/q;p' | sed '/^$/d' | awk '{ print $1 }' > /tmp/podman-system.cmd
man docs/podman-system.1 | sed -n -Ee '0,/COMMANDS/d'  -e 's/^[[:space:]]*//' -e '/^SEE ALSO/q;p' | grep podman | cut -f1 -d' ' | sed 's/^.//' > /tmp/podman-system.man
echo diff -B -u /tmp/podman-system.cmd /tmp/podman-system.man
diff -B -u /tmp/podman-system.cmd /tmp/podman-system.man

./bin/podman play --help | sed -n -e '0,/Available Commands/d' -e '/^Flags/q;p' | sed '/^$/d' | awk '{ print $1 }' > /tmp/podman-play.cmd
man docs/podman-play.1 | sed -n -Ee '0,/COMMANDS/d'  -e 's/^[[:space:]]*//' -e '/^SEE ALSO/q;p' | grep podman | cut -f1 -d' ' | sed 's/^.//' > /tmp/podman-play.man
echo diff -B -u /tmp/podman-play.cmd /tmp/podman-play.man
diff -B -u /tmp/podman-play.cmd /tmp/podman-play.man

./bin/podman generate --help | sed -n -e '0,/Available Commands/d' -e '/^Flags/q;p' | sed '/^$/d' | awk '{ print $1 }' > /tmp/podman-generate.cmd
man docs/podman-generate.1 | sed -n -Ee '0,/COMMANDS/d'  -e 's/^[[:space:]]*//' -e '/^SEE ALSO/q;p' | grep podman | cut -f1 -d' ' | sed 's/^.//' > /tmp/podman-generate.man
echo diff -B -u /tmp/podman-generate.cmd /tmp/podman-generate.man
diff -B -u /tmp/podman-generate.cmd /tmp/podman-generate.man

./bin/podman pod --help | sed -n -e '0,/Available Commands/d' -e '/^Flags/q;p' | sed '/^$/d' | awk '{ print $1 }' > /tmp/podman-pod.cmd
man docs/podman-pod.1 | sed -n -Ee '0,/COMMANDS/d'  -e 's/^[[:space:]]*//' -e '/^SEE ALSO/q;p' | grep podman | cut -f1 -d' ' | sed 's/^.//' > /tmp/podman-pod.man
echo diff -B -u /tmp/podman-pod.cmd /tmp/podman-pod.man
diff -B -u /tmp/podman-pod.cmd /tmp/podman-pod.man

./bin/podman volume --help | sed -n -e '0,/Available Commands/d' -e '/^Flags/q;p' | sed '/^$/d' | awk '{ print $1 }' > /tmp/podman-volume.cmd
man docs/podman-volume.1 | sed -n -Ee '0,/COMMANDS/d'  -e 's/^[[:space:]]*//' -e '/^SEE ALSO/q;p' | grep podman | cut -f1 -d' ' | sed 's/^.//' > /tmp/podman-volume.man
echo diff -B -u /tmp/podman-volume.cmd /tmp/podman-volume.man
diff -B -u /tmp/podman-volume.cmd /tmp/podman-volume.man
