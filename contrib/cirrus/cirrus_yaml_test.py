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
    SUCCESS_RELEASE = set(['success', 'release'])

    def setUp(self):
        super().setUp()
        self.ALL_TASK_NAMES = set([key.replace('_task', '')
                                   for key, _ in self.CIRRUS_YAML.items()
                                   if key.endswith('_task')])

    def test_dicts(self):
        """Expected dictionaries are present and non-empty"""
        for name in ('success_task', 'release_task'):
            # tests all names then show specific failures
            with self.subTest(name=name):
                self.assertIn(name, self.CIRRUS_YAML)
                self.assertIn(name.replace('_task', ''), self.ALL_TASK_NAMES)
                self.assertIn('depends_on', self.CIRRUS_YAML[name])
                self.assertGreater(len(self.CIRRUS_YAML[name]['depends_on']), 0)

    def _check_dep(self, name, task_name, deps):
        # name includes '_task' suffix, task_name does not
        msg=('Please add "{0}" to the "depends_on" list in "{1}"'
             "".format(task_name, name))
        self.assertIn(task_name, deps, msg=msg)

    def test_depends(self):
        """Success and Release tasks depend on all other tasks"""
        for name in ('success_task', 'release_task'):
            deps = set(self.CIRRUS_YAML[name]['depends_on'])
            for task_name in self.ALL_TASK_NAMES - self.SUCCESS_RELEASE:
                with self.subTest(name=name, task_name=task_name):
                    self._check_dep(name, task_name, deps)

    def test_release(self):
        """Release task must always execute last"""
        deps = set(self.CIRRUS_YAML['release_task']['depends_on'])
        self._check_dep('release_task', 'success', deps)


if __name__ == "__main__":
    unittest.main()
