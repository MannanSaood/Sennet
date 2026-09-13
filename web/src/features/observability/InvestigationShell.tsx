import { useEffect, useState } from 'react';
import { RefreshCw, Search, Share2 } from 'lucide-react';
import { ranges, useInvestigationState } from './state';

export function InvestigationShell({ children, onRefresh, pending = false }: { children: React.ReactNode; onRefresh?: () => void; pending?: boolean }) {
  const { state, update } = useInvestigationState(); const [query, setQuery] = useState(state.search); const [copied, setCopied] = useState(false);
  useEffect(() => setQuery(state.search), [state.search]);
  const share = async () => { await navigator.clipboard.writeText(window.location.href); setCopied(true); window.setTimeout(() => setCopied(false), 1500); };
  return <><div className="scope-bar" aria-label="Investigation scope">
    <form onSubmit={e => { e.preventDefault(); update({ q: query }); }}><Search size={15}/><input aria-label="Query" placeholder="Search telemetry…" value={query} onChange={e => setQuery(e.target.value)}/><button>Run</button></form>
    <select aria-label="Time range" value={state.range} onChange={e => update({ range: e.target.value })}>{ranges.map(([value,label]) => <option key={value} value={value}>{label}</option>)}</select>
    <select aria-label="Timezone" value={state.timezone} onChange={e => update({ timezone: e.target.value })}><option value="UTC">UTC</option><option value={Intl.DateTimeFormat().resolvedOptions().timeZone}>Local · {Intl.DateTimeFormat().resolvedOptions().timeZone}</option></select>
    <input aria-label="Environment" placeholder="All environments" value={state.environment} onChange={e => update({ environment: e.target.value })}/>
    <input aria-label="Service" placeholder="All services" value={state.service} onChange={e => update({ service: e.target.value })}/>
    <button aria-label="Refresh" disabled={pending} onClick={onRefresh}><RefreshCw size={15}/></button><button onClick={() => void share()}><Share2 size={15}/>{copied ? 'Copied' : 'Share'}</button>
  </div>{children}</>;
}
