import unittest
from harmony_click_check import parse_result, select_device


class ClickCheckTest(unittest.TestCase):
    def test_missing_or_ambiguous_device_requires_selection(self):
        for value in ('', '[Empty]\n', 'Connect server failed\n', 'first\nsecond\n'):
            with self.assertRaises(ValueError):
                select_device(value)
        self.assertEqual(select_device('first\n'), 'first')
        self.assertEqual(select_device('first\nsecond\n', 'second'), 'second')
        with self.assertRaises(ValueError):
            select_device('first\n', 'missing')

    def test_only_real_response_metric_determines_result(self):
        self.assertTrue(parse_result('ResponseTime:250ms\nCompleteTime:593ms')['passed'])
        self.assertFalse(parse_result('ResponseTime:319ms')['passed'])
        self.assertFalse(parse_result('ResponseTime:100.1ms', 100)['passed'])
        for value in ('CompleteTime:41ms', 'ResponseTime:0ms', 'ResponseTime:-1ms', 'ResponseTime:NaNms',
                      'not found', 'ResponseTime:20ms\nResponseTime:21ms'):
            with self.assertRaises(ValueError):
                parse_result(value)


if __name__ == '__main__':
    unittest.main()
