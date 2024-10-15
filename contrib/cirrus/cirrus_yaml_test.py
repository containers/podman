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
    # All tasks must be listed as a dependency of one/more of these tasks
    SUCCESS_DEPS_EXCLUDE = set(['build_success', 'success'])
    # Tasks which do not influence any success aggregator (above)
    NONSUCCESS_TASKS = set(['artifacts', 'release', 'release_test'])

    def setUp(self):
        super().setUp()
        self.ALL_TASK_NAMES = set([key.replace('_task', '')
                                   for key, _ in self.CIRRUS_YAML.items()
                                   if key.endswith('_task')])

    def test_dicts(self):
        """Specific tasks exist and always have non-empty depends_on"""
        for task_name in self.SUCCESS_DEPS_EXCLUDE | self.NONSUCCESS_TASKS:
            with self.subTest(task_name=task_name):
                msg = ('Expecting to find a "{0}" task'.format(task_name))
                self.assertIn(task_name, self.ALL_TASK_NAMES, msg=msg)
                task = self.CIRRUS_YAML[task_name + '_task']
                self.assertGreater(len(task['depends_on']), 0)

    def test_task(self):
        """There is no task named 'task'"""
        self.assertNotIn('task', self.ALL_TASK_NAMES)

    def test_depends(self):
        """Success aggregator tasks contain dependencies for all other tasks"""
        success_deps = set()
        for task_name in self.SUCCESS_DEPS_EXCLUDE:
            success_deps |= set(self.CIRRUS_YAML[task_name + '_task']['depends_on'])
        for task_name in self.ALL_TASK_NAMES - self.SUCCESS_DEPS_EXCLUDE - self.NONSUCCESS_TASKS:
            with self.subTest(task_name=task_name):
                msg=('No success aggregation task depends_on "{0}"'.format(task_name))
                self.assertIn(task_name, success_deps, msg=msg)

    def test_duplicate_depends(self):
        """Check for duplicate names in depends_on"""
        for task_name in self.ALL_TASK_NAMES:
            task = self.CIRRUS_YAML[task_name + '_task']
            if 'depends_on' in task:
                depends_on = task['depends_on']
                seen = set()
                for x in depends_on:
                    self.assertNotIn(x,seen, msg=f"depends_on contains duplicated names in task {task_name}")
                    seen.add(x)

    def test_only_if(self):
        """2024-07 PR#23174: ugly but necessary duplication in only_if conditions. Prevent typos or unwanted changes."""
        # N/B: This giant string is white space sensitive, take care when updating/modifying
        beginning = ("$CIRRUS_PR == '' || $CIRRUS_CHANGE_TITLE =~ '.*CI:ALL.*' || changesInclude('.cirrus.yml',"
                     " 'Makefile', 'contrib/cirrus/**', 'vendor/**', 'test/tools/**', 'test/registries*.conf',"
                     " 'hack/**', 'version/rawversion/*') || ")
        real_source_changes = " || (changesInclude('**/*.go', '**/*.c', '**/*.h') && !changesIncludeOnly('test/**', 'pkg/machine/e2e/**'))"

        for task_name in self.ALL_TASK_NAMES:
            task = self.CIRRUS_YAML[task_name + '_task']
            if 'only_if' in task:
                only_if = task['only_if']
                if 'changesInclude' in only_if:
                    msg = ('{0}: invalid only_if'.format(task_name))
                    self.assertEqual(only_if[:len(beginning)], beginning, msg=msg+": beginning part is wrong")
                    if 'changesIncludeOnly' in only_if:
                        self.assertEqual(only_if[len(only_if)-len(real_source_changes):], real_source_changes, msg=msg+": changesIncludeOnly() part is wrong")

    def not_task(self):
        """Ensure no task is named 'task'"""
        self.assertNotIn('task', self.ALL_TASK_NAMES)

if __name__ == "__main__":
    unittest.main()
