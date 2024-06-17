systemd tests

```bash
$ sudo CLEANUP=true \
       VERBOSE=false \
       NETWORK_NAME="${NETNAME}" \
       VOLUME_NAME="${VOLNAME}" \
       IMAGE_COMMAND="/sbin/init" \
       IMAGE_NAME="ubi8/ubi-init" \
       NUMBER_OF_CONTAINERS="1" \
       SYSTEMD_TIMEOUTSTOPSEC="INFINITY" \
       ./podman-stressor
```

Output:
```bash
$ ./is-TimeoutStopSec_infinity_works
[ INFO ] Cleaning any file from previous tests...
[ INFO ] Executing test: systemd TimeoutStopSec=Infinity
[ INFO ]
[ INFO ] This test was executed in the following criteria:
[ INFO ]
[ INFO ] Date: 2024-05-29 14:50:43 EDT
[ INFO ] System information:
[ INFO ] 	 - Fedora Linux 39 (Workstation Edition)
[ INFO ]
[ INFO ] RPM(s):
[ INFO ] 	 - systemd-254.9-1.fc39.x86_64
[ INFO ]
[ PASS ] systemd TimeoutStopSec=inifiny is working as expected
```

```
$ sudo CLEANUP=false \
       VERBOSE=false \
       NETWORK_NAME="my_network" \
       VOLUME_NAME="my_volume" \
       IMAGE_COMMAND="sleep 3600" \
       IMAGE_NAME="quay.io/centos-sig-automotive/automotive-osbuild" \
       NUMBER_OF_CONTAINERS="1" \
       SERVICE_MUST_BE_DISABLED="podman" \
       ./podman-stressor
```

Output:
```bash
$ ./is-service-disabled
[ INFO ] Cleaning any file from previous tests...
[ INFO ] Executing test: service must be disabled in the distro: [podman]
[ INFO ]
[ INFO ] This test was executed in the following criteria:
[ INFO ]
[ INFO ] Date: 2024-05-29 14:49:14 EDT
[ INFO ] System information:
[ INFO ] 	 - Fedora Linux 39 (Workstation Edition)
[ INFO ]
[ INFO ] RPM(s):
[ INFO ] 	 - systemd-254.9-1.fc39.x86_64
[ INFO ]
[ PASS ] service podman is DISABLED
```

```
   sudo CLEANUP=false \
         VERBOSE=false \
         NETWORK_NAME="my_network" \
         VOLUME_NAME="my_volume" \
         IMAGE_COMMAND="sleep 3600" \
         IMAGE_NAME="quay.io/centos-sig-automotive/automotive-osbuild" \
         NUMBER_OF_CONTAINERS="1" \
         SERVICE_MUST_BE_ENABLED="bluechi-controller,bluechi-agent" \
         ./podman-stressor
```

Output:
```bash
$ ./is-service-enabled
[ INFO ] Cleaning any file from previous tests...
[ INFO ] Executing test: service must be disabled in the distro: [podman]
[ INFO ]
[ INFO ] This test was executed in the following criteria:
[ INFO ]
[ INFO ] Date: 2024-05-29 14:49:59 EDT
[ INFO ] System information:
[ INFO ] 	 - Fedora Linux 39 (Workstation Edition)
[ INFO ]
[ INFO ] RPM(s):
[ INFO ] 	 - systemd-254.9-1.fc39.x86_64
[ INFO ]
[ FAIL ] service bluechi-controller is disabled
[ FAIL ] service bluechi-agent is disabled
```
