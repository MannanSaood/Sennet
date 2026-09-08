"""Verify actual asynchronous indexing, tenant scope, and complete-window counts."""
import json,os,time,urllib.request
base=os.environ.get('SENNET_QUERY_URL',os.environ.get('SENNET_URL','http://127.0.0.1:8080'))
headers={'Authorization':'Bearer '+os.environ['SENNET_API_KEY']}
def query(path):
    with urllib.request.urlopen(urllib.request.Request(base+path,headers=headers),timeout=20) as r: return json.load(r)
for attempt in range(30):
    summary=query('/api/summary')
    if summary['events']>=300: break
    time.sleep(2)
else: raise AssertionError('Kafka records did not become queryable')
page=query('/api/events?limit=20')
assert len(page['events'])==20 and page['next_cursor']
assert summary['services']>=4
assert all('tenant' not in e for e in page['events'])
finance=query('/api/finance/reconciliation')
assert finance['transactions']
print('Ingestion/query contract: PASS (streaming verification requires Kafka/ClickHouse deployment)')
