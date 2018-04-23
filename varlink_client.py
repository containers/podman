from varlink import (Client, VarlinkError)
import json

address = "unix:/run/podman/io.projectatomic.podman"

with Client(address=address) as client:
    podman = client.open('io.projectatomic.podman')
    response = podman.GetVersion()
    print(json.dumps(response, indent=4, separators=(',', ': ')))