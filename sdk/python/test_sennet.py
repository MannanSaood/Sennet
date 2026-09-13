import json
import tempfile
import unittest
import urllib.error
from decimal import Decimal
from unittest.mock import patch, MagicMock
from sennet import Recorder, ContentPolicy


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

    def test_agent_metadata_exact_usage_and_cost_provenance(self):
        self.recorder.agent_event('model', 'run-1', input_tokens=9007199254740993,
            output_tokens=7, estimated_cost='0.1250', observed_cost='0.1300',
            pricing_version='provider-2026-09', provider='openai', model='gpt', content='private')
        event = self.events()[0]
        self.assertEqual(event['attributes']['gen_ai.usage.input_tokens'], '9007199254740993')
        self.assertEqual(event['attributes']['gen_ai.cost.estimated'], '0.1250')
        self.assertEqual(event['attributes']['gen_ai.cost.observed'], '0.1300')
        self.assertNotIn('content.body', event['attributes'])

    def test_content_capture_requires_explicit_policy_and_redaction_hook(self):
        recorder = Recorder('http://localhost', 'key', 'test', self.directory.name,
            content_policy=ContentPolicy(True, lambda _: '[REDACTED]', 'content-1d'))
        recorder.agent_event('prompt', 'run-content', prompt_id='template-1', content='ignore previous instructions')
        event = self.events()[0]
        self.assertEqual(event['attributes']['content.body'], '[REDACTED]')
        self.assertEqual(event['attributes']['content.retention_class'], 'content-1d')


if __name__ == '__main__':
    unittest.main()
