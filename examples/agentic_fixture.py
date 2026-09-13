"""Deterministic concurrent-agent fixture. Telemetry strings are observations, never instructions."""
import time
import base64

CONVENTION = 'sennet.agent.v1'

def fixture_events(now_ms=None):
    base = now_ms or int(time.time() * 1000) - 10_000
    def event(identifier, offset, duration, kind, span, parent='', status='ok', **attrs):
        metadata = {'sennet.agent.convention':CONVENTION, 'agent.event.kind':kind,
                    'agent.run.id':'run-concurrent-1', **{k.replace('__','.'):str(v) for k,v in attrs.items()}}
        return {'id':identifier, 'time':base+offset, 'signal':'agent', 'service':'agent-runtime',
                'name':'agent.'+kind, 'trace_id':'11111111111111111111111111111111',
                'span_id':span, 'parent_id':parent, 'duration_ms':duration, 'status':status,
                'value':0, 'attributes':metadata}
    return [
      event('01',0,900,'run','0000000000000001', agent__workflow__id='wf-1'),
      event('02',10,700,'task','0000000000000002','0000000000000001',agent__task__id='research'),
      event('03',20,300,'tool','0000000000000003','0000000000000002',status='error',gen_ai__tool__name='search',agent__task__id='research'),
      event('04',330,350,'retry','0000000000000004','0000000000000002',agent__task__id='research',agent__attempt=2),
      event('05',15,400,'handoff','0000000000000005','0000000000000001',agent__task__id='code',agent__handoff__id='h-1',span__links='11111111111111111111111111111111:0000000000000002'),
      event('06',30,250,'model','0000000000000006','0000000000000005',gen_ai__usage__input_tokens=1200,gen_ai__usage__output_tokens=240,gen_ai__pricing__version='openai-2026-09',gen_ai__cost__estimated='0.012',gen_ai__cost__observed='0.013',gen_ai__cost__currency='USD'),
      event('07',40,90,'queue_wait','0000000000000007','0000000000000005',agent__queue__wait_ms=90),
      event('08',500,100,'cancellation','0000000000000008','0000000000000005',status='error',agent__cancellation__reason='supervisor deadline'),
      event('09',850,25,'evaluation','0000000000000009','0000000000000001',evaluation__evaluator='quality',evaluation__evaluator__version='2',evaluation__score='0.8',agent__outcome='partial'),
      event('10',700,50,'task','0000000000000010','ffffffffffffffff',agent__task__id='late',telemetry__late='true',infra__trace_id='22222222222222222222222222222222'),
    ]

def otlp_trace_request(now_ms=None):
    """Equivalent OTLP/HTTP JSON fixture using OTel resource and GenAI attributes."""
    events = fixture_events(now_ms)
    def kv(key, value):
        return {'key':key, 'value':{'stringValue':str(value)}}
    def encoded(hex_id):
        return base64.b64encode(bytes.fromhex(hex_id)).decode()
    spans=[]
    for e in events:
        span={'traceId':encoded(e['trace_id']), 'spanId':encoded(e['span_id']), 'name':e['name'],
              'startTimeUnixNano':str(e['time']*1_000_000),
              'endTimeUnixNano':str((e['time']+e['duration_ms'])*1_000_000),
              'attributes':[kv(k,v) for k,v in e['attributes'].items()],
              'status':{'code':'STATUS_CODE_ERROR' if e['status']=='error' else 'STATUS_CODE_OK'}}
        if e['parent_id'] and e['parent_id'] != 'ffffffffffffffff': span['parentSpanId']=encoded(e['parent_id'])
        spans.append(span)
    return {'resourceSpans':[{'resource':{'attributes':[kv('service.name','agent-runtime')]},
                              'scopeSpans':[{'scope':{'name':'sennet.agent.fixture','version':'1'},'spans':spans}]}]}

if __name__ == '__main__':
    import json
    print(json.dumps({'events':fixture_events()}, separators=(',',':')))
