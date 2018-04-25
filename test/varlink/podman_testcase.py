"""Custom TestCase for varlink/podman."""
import os
import unittest

import varlink


class PodmanTestCase(unittest.TestCase):
    """Provides varlink setup for podman."""

    def __init__(self, *args, **kwargs):
        """Initialize class by calling parent."""
        super(PodmanTestCase, self).__init__(*args, **kwargs)
        self.address = os.environ.get(
            'PODMAN_HOST',
            'unix:/run/podman/io.projectatomic.podman')

    def setUp(self):
        """Set up the varlink/podman fixture before each test."""
        super(PodmanTestCase, self).setUp()
        self.client = varlink.Client(
            address=self.address)
        self.podman = self.client.open('io.projectatomic.podman')

    def tearDown(self):
        """Deconstruct the varlink/podman fixture after each test."""
        super(PodmanTestCase, self).tearDown()
        self.podman.close()
