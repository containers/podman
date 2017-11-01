In terminal 1:
```
sudo ./crio
```

In terminal 2:
```
sudo ./crioctl runtimeversion

sudo rm -rf /var/lib/containers/storage/sandboxes/podsandbox1
sudo ./crioctl pod run --config testdata/sandbox_config.json

sudo rm -rf /var/lib/containers/storage/containers/container1
sudo ./crioctl container create --pod podsandbox1 --config testdata/container_config.json
```
