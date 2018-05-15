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
                '2018-05-08 14:12:53.797795191 -0700 MST'
        ]:
            actual = podman.datetime_parse(v)
            self.assertEqual(actual, expected)

            podman.datetime_parse(datetime.datetime.now().isoformat())

    def test_parse_fail(self):
        # chronologist humor: '1752-09-05T12:00:00.000000-0000' also not
        #   handled correctly by python for my locale.
        for v in [
                '1752-9-5',
                '1752-09-05',
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
