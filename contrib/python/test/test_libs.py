import datetime
import unittest

import podman


class TestLibs(unittest.TestCase):
    def setUp(self):
        pass

    def tearDown(self):
        pass

    def test_parse(self):
        expected = datetime.datetime.strptime(
            '2018-05-08T14:12:53.797795-0700', '%Y-%m-%dT%H:%M:%S.%f%z')
        for v in [
                '2018-05-08T14:12:53.797795191-07:00',
                '2018-05-08T14:12:53.797795-07:00',
                '2018-05-08T14:12:53.797795-0700',
                '2018-05-08 14:12:53.797795191 -0700 MST',
        ]:
            actual = podman.datetime_parse(v)
            self.assertEqual(actual, expected)

        expected = datetime.datetime.strptime(
            '2018-05-08T14:12:53.797795-0000', '%Y-%m-%dT%H:%M:%S.%f%z')
        for v in [
                '2018-05-08T14:12:53.797795191Z',
                '2018-05-08T14:12:53.797795191z',
        ]:
            actual = podman.datetime_parse(v)
            self.assertEqual(actual, expected)

        actual = podman.datetime_parse(datetime.datetime.now().isoformat())
        self.assertIsNotNone(actual)

    def test_parse_fail(self):
        for v in [
                'There is no time here.',
        ]:
            with self.assertRaises(ValueError):
                podman.datetime_parse(v)

    def test_format(self):
        expected = '2018-05-08T18:24:52.753227-07:00'
        dt = podman.datetime_parse(expected)
        actual = podman.datetime_format(dt)
        self.assertEqual(actual, expected)


if __name__ == '__main__':
    unittest.main()
