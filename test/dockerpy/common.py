import docker
from docker import Client


def get_client():
   return docker.Client(base_url="unix:/run/podman/podman.sock")
