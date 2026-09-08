import json
import tempfile
import unittest
import urllib.error
from decimal import Decimal
from unittest.mock import patch, MagicMock
from sennet import Recorder


class RecorderTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.recorder = Recorder('http://localhost:8080', 'evaluation', 'test', self.directory.name)

    def events(self):
        return [json.loads(p.read_text())['events'][0] for p in self.recorder.spool.glob('*.json')]

    def test_nested_spans_propagate_and_reset_context(self):
        with self.recorder.span('parent') as parent:
            with self.recorder.span('child'):
                pass
        with self.recorder.span('separate'):
            pass
        events = {e['name']: e for e in self.events()}
        self.assertEqual(events['child']['parent_id'], parent['span_id'])
        self.assertEqual(events['child']['trace_id'], parent['trace_id'])
        self.assertNotEqual(events['separate']['trace_id'], parent['trace_id'])

    def test_transport_failure_retains_identical_event_until_ack(self):
        identifier = self.recorder.record('log', 'hello')
        with patch('urllib.request.urlopen', side_effect=urllib.error.URLError('offline')):
            with self.assertRaises(urllib.error.URLError): self.recorder.flush()
        self.assertEqual(self.events()[0]['id'], identifier)
        response = MagicMock()
        response.__enter__.return_value.status = 202
        with patch('urllib.request.urlopen', return_value=response):
            self.assertEqual(self.recorder.flush(), 1)
        self.assertEqual(self.events(), [])

    def test_exact_financial_amount_and_float_rejection(self):
        self.recorder.financial_event('tx', 'settled', Decimal('9007199254740993.0001'), 'USD', 1, 'test')
        self.assertEqual(self.events()[0]['attributes']['amount'], '9007199254740993.0001')
        with self.assertRaises(TypeError):
            self.recorder.financial_event('tx', 'settled', 0.1, 'USD', 1, 'test')

    def test_full_spool_does_not_replace_business_exception(self):
        self.recorder.max_bytes = 0
        with self.assertLogs('sennet', level='ERROR'):
            with self.assertRaisesRegex(ValueError, 'business failure'):
                with self.recorder.span('failed'):
                    raise ValueError('business failure')


if __name__ == '__main__':
    unittest.main()
