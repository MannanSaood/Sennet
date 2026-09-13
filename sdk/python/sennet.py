"""Sennet instrumentation with context propagation and a durable local outbox.
No third-party dependencies. Workload identity is supplied explicitly.
"""
import contextlib, contextvars, json, logging, os, pathlib, threading, time, uuid
import urllib.error, urllib.request
from decimal import Decimal
_current = contextvars.ContextVar('sennet_span', default=None)
AGENT_CONVENTION_VERSION = 'sennet.agent.v1'

class ContentPolicy:
    """Metadata-only by default. A hook may further redact metadata before spooling."""
    def __init__(self, capture_content=False, redact=None, retention_class='metadata-30d'):
        self.capture_content = bool(capture_content)
        self.redact = redact or (lambda value: value)
        self.retention_class = retention_class

class Recorder:
    def __init__(self, endpoint, key, service, spool='./sennet-outbox', max_bytes=20*1024*1024,
                 content_policy=None):
        self.endpoint, self.key, self.service = endpoint.rstrip('/'), key, service
        self.spool = pathlib.Path(spool)
        self.spool.mkdir(parents=True, exist_ok=True)
        self.max_bytes = max_bytes
        self.content_policy = content_policy or ContentPolicy()
        self._lock = threading.Lock()

    def record(self, signal, name, attributes=None, **fields):
        event = dict(id=uuid.uuid4().hex, time=int(time.time()*1000), signal=signal,
                     name=name, service=self.service, status='ok', duration_ms=0,
                     value=0, attributes={str(k):str(v) for k,v in (attributes or {}).items()})
        event.update(fields)
        payload = json.dumps({'events':[event]}, allow_nan=False).encode()
        with self._lock:
            if sum(p.stat().st_size for p in self.spool.iterdir() if p.is_file())+len(payload)>self.max_bytes:
                raise BufferError('Sennet outbox full; observation was not accepted')
            temporary = self.spool / (event['id']+'.tmp')
            with temporary.open('xb') as f:
                f.write(payload); f.flush(); os.fsync(f.fileno())
            temporary.replace(temporary.with_suffix('.json'))
        return event['id']

    def agent_event(self, kind, run_id, *, session_id='', workflow_id='', task_id='',
                    agent_id='', agent_version='', model='', provider='', model_version='',
                    prompt_id='', prompt_version='', tool_name='', tool_call_id='', handoff_id='',
                    attempt=1, queue_wait_ms=None, cancellation_reason='', input_tokens=None,
                    output_tokens=None, cached_tokens=None, pricing_version='', estimated_cost='',
                    observed_cost='', currency='', evaluator='', evaluator_version='', outcome='',
                    score='', links=None, metadata=None, content=None, event_id=None, **fields):
        """Record sennet.agent.v1 metadata. Exact token counts and monetary values stay strings."""
        if not run_id or kind not in {'run','session','workflow','task','agent','model','prompt','tool',
                                      'handoff','retry','queue_wait','cancellation','tokens','cost',
                                      'evaluation','outcome'}:
            raise ValueError('valid agent event kind and run_id required')
        attrs = {'sennet.agent.convention': AGENT_CONVENTION_VERSION, 'agent.event.kind': kind,
                 'agent.run.id': run_id, 'agent.attempt': str(attempt),
                 'content.captured': 'false', 'content.retention_class': self.content_policy.retention_class}
        values = {'agent.session.id':session_id, 'agent.workflow.id':workflow_id,
                  'agent.task.id':task_id, 'agent.id':agent_id, 'agent.version':agent_version,
                  'gen_ai.request.model':model, 'gen_ai.provider.name':provider,
                  'gen_ai.response.model':model_version, 'gen_ai.prompt.id':prompt_id,
                  'gen_ai.prompt.version':prompt_version, 'gen_ai.tool.name':tool_name,
                  'gen_ai.tool.call.id':tool_call_id, 'agent.handoff.id':handoff_id,
                  'agent.queue.wait_ms':queue_wait_ms, 'agent.cancellation.reason':cancellation_reason,
                  'gen_ai.usage.input_tokens':input_tokens, 'gen_ai.usage.output_tokens':output_tokens,
                  'gen_ai.usage.cached_tokens':cached_tokens, 'gen_ai.pricing.version':pricing_version,
                  'gen_ai.cost.estimated':str(estimated_cost), 'gen_ai.cost.observed':str(observed_cost),
                  'gen_ai.cost.currency':currency, 'evaluation.evaluator':evaluator,
                  'evaluation.evaluator.version':evaluator_version, 'evaluation.score':str(score),
                  'agent.outcome':outcome}
        attrs.update({k:str(v) for k,v in values.items() if v not in ('', None)})
        attrs.update({str(k):str(v) for k,v in (metadata or {}).items()})
        if links: attrs['span.links'] = ','.join(links)
        if content is not None and self.content_policy.capture_content:
            attrs['content.body'] = str(self.content_policy.redact(content))
            attrs['content.captured'] = 'true'
        if event_id is not None: fields['id'] = event_id
        return self.record('agent', 'agent.'+kind, attrs, **fields)

    @contextlib.contextmanager
    def agent_span(self, kind, run_id, **metadata):
        attributes = dict(metadata.pop('attributes', {}) or {})
        attributes.update({'sennet.agent.convention':AGENT_CONVENTION_VERSION,
                           'agent.event.kind':kind, 'agent.run.id':run_id})
        with self.span('agent.'+kind, attributes=attributes, signal='agent',
                       trace_id=metadata.pop('trace_id', None), parent_id=metadata.pop('parent_id', None)) as ctx:
            yield ctx

    @contextlib.contextmanager
    def span(self, name, attributes=None, signal='agent', trace_id=None, parent_id=None):
        parent = _current.get()
        trace = trace_id or (parent[0] if parent else uuid.uuid4().hex)
        span_id = uuid.uuid4().hex[:16]
        parent_id = parent_id or (parent[1] if parent else '')
        token = _current.set((trace,span_id))
        started = time.time_ns(); elapsed_start = time.perf_counter_ns(); status='ok'
        try:
            yield {'trace_id':trace,'span_id':span_id}
        except BaseException:
            status='error'
            raise
        finally:
            _current.reset(token)
            try:
                self.record(signal,name,attributes,trace_id=trace,span_id=span_id,parent_id=parent_id,
                            time=started//1000000,duration_ms=(time.perf_counter_ns()-elapsed_start)/1e6,status=status)
            except Exception:
                # Instrumentation must preserve the workload's own exception.
                logging.getLogger(__name__).exception('Sennet could not persist span %s', name)

    def financial_event(self, transaction_id, state, amount, currency, sequence, source):
        # Avoid accepting float quantities whose precision was already lost.
        if not isinstance(amount,(str,Decimal)):
            raise TypeError('amount must be a decimal string or Decimal')
        exact=Decimal(amount)
        if not exact.is_finite(): raise ValueError('amount must be finite')
        return self.record('finance',state,{'transaction_id':transaction_id,'state':state,
                           'amount':format(exact,'f'),'currency':currency,'sequence':sequence,'source':source})

    def flush(self, limit=100):
        """Retry on the next call after transient failure; never remove unacked data."""
        sent=0
        with self._lock:
            for file in sorted(self.spool.glob('*.json'))[:limit]:
                req=urllib.request.Request(self.endpoint+'/api/events',data=file.read_bytes(),
                    headers={'Authorization':'Bearer '+self.key,'Content-Type':'application/json'})
                try:
                    with urllib.request.urlopen(req,timeout=10) as response:
                        if response.status!=202: raise RuntimeError('No durable acknowledgement')
                except urllib.error.HTTPError as error:
                    if error.code in (400,413,422): file.rename(file.with_suffix('.rejected'))
                    raise
                file.unlink(); sent+=1
        return sent
