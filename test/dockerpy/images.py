import unittest
import docker

busybox = "docker.io/library/busybox:latest"

def get_client():
   return docker.DockerClient(base_url="unix://var/run/docker.sock")

class TestImages(unittest.TestCase):
    client = ""
    def test_get_image(self):
        client = get_client()
        bb = client.images.get("busybox")
        self.assertTrue(busybox in bb.tags)


if __name__ == '__main__':
    # Setup temporary space
    unittest.main()
