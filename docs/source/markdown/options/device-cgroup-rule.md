#### **--device-cgroup-rule**=*"type major:minor mode"*

Add a rule to the cgroup allowed devices list. The rule is expected to be in the format specified in the Linux kernel documentation (Documentation/cgroup-v1/devices.txt):
       - type: a (all), c (char), or b (block);
       - major and minor: either a number, or * for all;
       - mode: a composition of r (read), w (write), and m (mknod(2)).
