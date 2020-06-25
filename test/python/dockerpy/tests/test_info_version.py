import unittest

from . import common, constant

client = common.get_client()


class TestInfo_Version(unittest.TestCase):

    podman = None
    topContainerId = ""

    def setUp(self):
        super().setUp()
        common.restore_image_from_cache(self)
        TestInfo_Version.topContainerId = common.run_top_container()

    def tearDown(self):
        common.remove_all_containers()
        common.remove_all_images()
        return super().tearDown()

    @classmethod
    def setUpClass(cls):
        super().setUpClass()
        common.enable_sock(cls)

    @classmethod
    def tearDownClass(cls):
        common.terminate_connection(cls)
        return super().tearDownClass()

    def test_Info(self):
        self.assertIsNotNone(client.info())

    def test_info_container_details(self):
        info = client.info()
        self.assertEqual(info["Containers"], 1)
        client.create_container(image=constant.ALPINE)
        info = client.info()
        self.assertEqual(info["Containers"], 2)

    def test_version(self):
        self.assertIsNotNone(client.version())
