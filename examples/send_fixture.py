"""Explicit evaluation fixtures through real ingestion. Standard library only."""
import json, os, time, uuid, urllib.error, urllib.request
url = os.environ.get('SENNET_URL', 'http://127.0.0.1:8080')
key = os.environ['SENNET_SESSION_TOKEN']
now = int(time.time()*1000)
events=[]
for i in range(60):
    trace=uuid.uuid4().hex
    for j,(service,name,signal,duration) in enumerate([
        ('orchestrator','research.run','agent',480),('risk-agent','risk.evaluate','agent',180),
        ('pricing-api','GET /quotes','trace',42),('settlement','settlement.completed','finance',18)]):
        attrs={'environment':'evaluation','agent.id':service,'gen_ai.usage.input_tokens':str(120+i),'gen_ai.usage.output_tokens':str(40+i),'run.id':trace}
        if signal=='finance': attrs.update(transaction_id=f'eval-{i}',source_id='fixture-payments',account='evaluation-merchant',provider='evaluation-provider',state='settled',currency='USD',amount='1250.0000',sequence='3',receive_time_ms=str(now-(60-i)*10000+j*10+7),correction_version='0',**{'finance.schema':'sennet.finance.payment.v1'})
        events.append(dict(id=uuid.uuid4().hex,time=now-(60-i)*10000+j*10,signal=signal,service=service,name=name,trace_id=trace,span_id=f'{j+1:016x}',parent_id=f'{j:016x}' if j else '',duration_ms=duration,value=0,status='error' if i%9==0 and j==1 else 'ok',attributes=attrs))
    events.append(dict(id=uuid.uuid4().hex,time=now-(60-i)*10000,signal='flow',service='pricing-api',name='pricing → settlement',status='ok',duration_ms=0,value=2048,attributes={'source.service':'pricing-api','destination.service':'settlement','bytes':'2048','protocol':'TCP','environment':'evaluation'}))
headers={'Authorization':'Bearer '+key,'Content-Type':'application/json'}
if workspace := os.environ.get('SENNET_WORKSPACE'):
    headers['X-Sennet-Workspace'] = workspace
req=urllib.request.Request(url+'/api/events',data=json.dumps({'events':events}).encode(),headers=headers)
try:
    with urllib.request.urlopen(req,timeout=20) as response: print(response.read().decode())
except urllib.error.HTTPError as error:
    print(error.read().decode())
    raise
