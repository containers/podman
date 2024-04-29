#!/usr/bin/env python3

"""
Verify contents of .cirrus.yml meet specific expectations
"""

import sys
import os
import unittest
import yaml

# Assumes directory structure of this file relative to repo.
SCRIPT_DIRPATH = os.path.dirname(os.path.realpath(__file__))
REPO_ROOT = os.path.realpath(os.path.join(SCRIPT_DIRPATH, '../', '../'))


class TestCaseBase(unittest.TestCase):

    CIRRUS_YAML = None

    def setUp(self):
        with open(os.path.join(REPO_ROOT, '.cirrus.yml')) as cirrus_yaml:
            self.CIRRUS_YAML = yaml.safe_load(cirrus_yaml.read())


class TestDependsOn(TestCaseBase):

    ALL_TASK_NAMES = None
    SUCCESS_DEPS_EXCLUDE = set(['success', 'artifacts',
        'test_image_build', 'release', 'release_test'])

    def setUp(self):
        super().setUp()
        self.ALL_TASK_NAMES = set([key.replace('_task', '')
                                   for key, _ in self.CIRRUS_YAML.items()
                                   if key.endswith('_task')])

    def test_dicts(self):
        """Expected dictionaries are present and non-empty"""
        self.assertIn('success_task', self.CIRRUS_YAML)
        self.assertIn('success_task'.replace('_task', ''), self.ALL_TASK_NAMES)
        self.assertIn('depends_on', self.CIRRUS_YAML['success_task'])
        self.assertGreater(len(self.CIRRUS_YAML['success_task']['depends_on']), 0)

    def test_task(self):
        """There is no task named 'task'"""
        self.assertNotIn('task', self.ALL_TASK_NAMES)

    def test_depends(self):
        """Success task depends on all other tasks"""
        success_deps = set(self.CIRRUS_YAML['success_task']['depends_on'])
        for task_name in self.ALL_TASK_NAMES - self.SUCCESS_DEPS_EXCLUDE:
            with self.subTest(task_name=task_name):
                msg=('Please add "{0}" to the "depends_on" list in "success_task"'
                     "".format(task_name))
                self.assertIn(task_name, success_deps, msg=msg)

    def not_task(self):
        """Ensure no task is named 'task'"""
        self.assertNotIn('task', self.ALL_TASK_NAMES)

if __name__ == "__main__":
    unittest.main()
