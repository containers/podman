selinux tests

```
sudo CLEANUP=false \
     VERBOSE=false \
     NETWORK_NAME="my_network" \
     VOLUME_NAME="my_volume" \
     IMAGE_COMMAND="sleep 3600" \
     IMAGE_NAME="quay.io/centos-sig-automotive/automotive-osbuild" \
     NUMBER_OF_CONTAINERS="1" \
     SELINUX_STATUS_MUST_BE="Disabled" \
     ./podman-stressor
```

Output:
```bash
$ ./is-selinux-status-disabled
[ INFO ] Cleaning any file from previous tests...
[ INFO ] Executing test: SELinux must be [DISABLED]
[ INFO ]
[ INFO ] This test was executed in the following criteria:
[ INFO ]
[ INFO ] Date: 2024-05-29 14:55:17 EDT
[ INFO ] System information:
[ INFO ] 	 - Fedora Linux 39 (Workstation Edition)
[ INFO ]
[ INFO ] RPM(s):
[ INFO ] 	 - selinux-policy-39.4-1.fc39.noarch
[ INFO ] 	 - libselinux-3.5-5.fc39.x86_64
[ INFO ]
[ PASS ] SELinux status is DISABLED
```

```
sudo CLEANUP=false \
     VERBOSE=false \
     NETWORK_NAME="my_network" \
     VOLUME_NAME="my_volume" \
     IMAGE_COMMAND="sleep 3600" \
     IMAGE_NAME="quay.io/centos-sig-automotive/automotive-osbuild" \
     NUMBER_OF_CONTAINERS="1" \
     SELINUX_STATUS_MUST_BE="Enforcing" \
     ./podman-stressor
```
Output:
```bash
$ ./is-selinux-status-enforcing
[ INFO ] Cleaning any file from previous tests...
[ INFO ] Executing test: SELinux must be [ENFORCING]
[ INFO ]
[ INFO ] This test was executed in the following criteria:
[ INFO ]
[ INFO ] Date: 2024-05-29 14:56:38 EDT
[ INFO ] System information:
[ INFO ] 	 - Fedora Linux 39 (Workstation Edition)
[ INFO ]
[ INFO ] RPM(s):
[ INFO ] 	 - selinux-policy-39.4-1.fc39.noarch
[ INFO ] 	 - libselinux-3.5-5.fc39.x86_64
[ INFO ]
[ FAIL ] SELinux is NOT in ENFORCING mode in container test_container_1.
[ FAIL ] The current status is: DISABLED
```
