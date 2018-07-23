from __future__ import absolute_import

import unittest

from report import Report, ReportColumn


class TestReport(unittest.TestCase):
    def setUp(self):
        pass

    def test_report_column(self):
        rc = ReportColumn('k', 'v', 3)
        self.assertEqual(rc.key, 'k')
        self.assertEqual(rc.display, 'v')
        self.assertEqual(rc.width, 3)
        self.assertIsNone(rc.default)

        rc = ReportColumn('k', 'v', 3, 'd')
        self.assertEqual(rc.key, 'k')
        self.assertEqual(rc.display, 'v')
        self.assertEqual(rc.width, 3)
        self.assertEqual(rc.default, 'd')
