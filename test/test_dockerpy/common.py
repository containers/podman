import docker
import subprocess
import os
import sys
import time
from docker import Client
from . import constant

alpineDict = {
      "name":        "docker.io/library/alpine:latest",
		"shortName":   "alpine",
		"tarballName": "alpine.tar"}

def get_client():
   client = docker.Client(base_url="http://localhost:8080",timeout=15)
   return client

client = get_client()

def podman():
    binary = os.getenv("PODMAN_BINARY")
    if binary is None:
        binary = "bin/podman"
    return binary

def restore_image_from_cache(TestClass):
   alpineImage = os.path.join(constant.ImageCacheDir , alpineDict["tarballName"])
   if not os.path.exists(alpineImage):
      os.makedirs(constant.ImageCacheDir)
      client.pull(constant.ALPINE)
      response = client.get_image(constant.ALPINE)
      image_tar = open(alpineImage,mode="wb")
      image_tar.write(response.data)
      image_tar.close()
   else :
      TestClass.podman = subprocess.run(
      [
            podman(), "load", "-i", alpineImage
      ],
      shell=False,
      stdin=subprocess.DEVNULL,
      stdout=subprocess.DEVNULL,
      stderr=subprocess.DEVNULL,
   )

def run_top_container():
   c = client.create_container(image=constant.ALPINE,command='/bin/sleep 5',name=constant.TOP)
   client.start(container=c.get("Id"))
   return c.get("Id")

def enable_sock(TestClass):
   TestClass.podman = subprocess.Popen(
      [
            podman(), "system", "service", "tcp:localhost:8080",
            "--log-level=debug", "--time=0"
      ],
      shell=False,
      stdin=subprocess.DEVNULL,
      stdout=subprocess.DEVNULL,
      stderr=subprocess.DEVNULL,
   )
   time.sleep(2)

def terminate_connection(TestClass):
   TestClass.podman.terminate()
   stdout, stderr = TestClass.podman.communicate(timeout=0.5)
   if stdout:
      print("\nService Stdout:\n" + stdout.decode('utf-8'))
   if stderr:
      print("\nService Stderr:\n" + stderr.decode('utf-8'))

   if TestClass.podman.returncode > 0:
      sys.stderr.write("podman exited with error code {}\n".format(
            TestClass.podman.returncode))
      sys.exit(2)

def remove_all_containers():
   containers = client.containers(quiet=True)
   for c in containers:
      client.remove_container(container=c.get("Id"), force=True)

def remove_all_images():
   allImages = client.images()
   for image in allImages:
      client.remove_image(image,force=True)
