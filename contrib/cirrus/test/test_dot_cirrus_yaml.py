#!/bin/env python3

import sys
import os
import os.path
import unittest
import warnings
import yaml

class TestCaseBase(unittest.TestCase):

    SCRIPT_PATH = os.path.realpath((os.path.dirname(sys.argv[0])))
    CIRRUS_WORKING_DIR = os.environ.get('CIRRUS_WORKING_DIR',
                                        '{0}/../../../'.format(SCRIPT_PATH))

    def setUp(self):
        os.chdir(self.CIRRUS_WORKING_DIR)


class TestCirrusYAML(TestCaseBase):

    IMAGE_NAME_SUFFIX = '_CACHE_IMAGE_NAME'
    ACTIVE_IMAGES_NAME = 'ACTIVE_CACHE_IMAGE_NAMES'

    def setUp(self):
        TestCirrusYAML._cirrus = None
        super().setUp()

    @property
    def cirrus(self):
        if TestCirrusYAML._cirrus is None:
            with warnings.catch_warnings():
                warnings.filterwarnings("ignore",category=DeprecationWarning)
                with open('.cirrus.yml', "r") as dot_cirrus_dot_yaml:
                    TestCirrusYAML._cirrus = yaml.load(dot_cirrus_dot_yaml)
        return TestCirrusYAML._cirrus

    def _assert_get_cache_image_names(self, env):
        inames = set([key for key in env.keys()
                      if key.endswith(self.IMAGE_NAME_SUFFIX)])
        self.assertNotEqual(inames, set())

        ivalues = set([value for key, value in env.items()
                       if key in inames])
        self.assertNotEqual(ivalues, set())
        return ivalues

    def _assert_get_subdct(self, key, dct):
        self.assertIn(key, dct)
        return dct[key]

    def test_parse_yaml(self):
        self.assertIsInstance(self.cirrus, dict)

    def test_active_cache_image_names(self):
        env = self._assert_get_subdct('env', self.cirrus)
        acin = self._assert_get_subdct(self.ACTIVE_IMAGES_NAME, env)

        for ivalue in self._assert_get_cache_image_names(env):
            self.assertIn(ivalue, acin,
                          "The '{}' sub-key of 'env' should contain this among"
                          " its space-separated values."
                          "".format(self.ACTIVE_IMAGES_NAME))


    def test_cache_image_names_active(self):
        env = self._assert_get_subdct('env', self.cirrus)
        ivalues = self._assert_get_cache_image_names(env)

        for avalue in set(self._assert_get_subdct(self.ACTIVE_IMAGES_NAME, env).split()):
            self.assertIn(avalue, ivalues,
                          "All space-separated values in the '{}' sub-key"
                          " of 'env' must also be used in a key with a '{}' suffix."
                          "".format(self.ACTIVE_IMAGES_NAME, self.IMAGE_NAME_SUFFIX))


if __name__ == '__main__':
    unittest.main(failfast=True, catchbreak=True, verbosity=0)
