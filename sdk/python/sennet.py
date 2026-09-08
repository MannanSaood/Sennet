"""Sennet instrumentation with context propagation and a durable local outbox.
No third-party dependencies. Workload identity is supplied explicitly.
"""
import contextlib, contextvars, json, logging, os, pathlib, threading, time, uuid
import urllib.error, urllib.request
from decimal import Decimal
_current = contextvars.ContextVar('sennet_span', default=None)

class Recorder:
    def __init__(self, endpoint, key, service, spool='./sennet-outbox', max_bytes=20*1024*1024):
        self.endpoint, self.key, self.service = endpoint.rstrip('/'), key, service
        self.spool = pathlib.Path(spool)
        self.spool.mkdir(parents=True, exist_ok=True)
        self.max_bytes = max_bytes
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
