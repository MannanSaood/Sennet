import { AlertTriangle, Ban, Clock3, LoaderCircle, RadioTower } from 'lucide-react';

export function QueryState({ kind, title, detail, retry }: { kind: 'loading'|'empty'|'denied'|'stale'|'partial'|'failed'; title?: string; detail?: string; retry?: () => void }) {
  const Icon = kind === 'loading' ? LoaderCircle : kind === 'denied' ? Ban : kind === 'stale' ? Clock3 : kind === 'empty' ? RadioTower : AlertTriangle;
  const defaults = { loading: ['Loading telemetry', 'The active request can be cancelled by changing filters.'], empty: ['No telemetry matched', 'Expand the window or clear a filter. No demo data was substituted.'], denied: ['Access denied', 'Your workspace role cannot read this resource.'], stale: ['Results may be stale', 'The latest refresh failed; the last successful result remains visible.'], partial: ['Partial result', 'A backend query budget limited this result.'], failed: ['Query failed', 'The backend did not return telemetry.'] }[kind];
  return <div className={`query-state ${kind}`} role={kind === 'failed' || kind === 'denied' ? 'alert' : 'status'}><Icon size={18} aria-hidden/><div><strong>{title || defaults[0]}</strong><p>{detail || defaults[1]}</p></div>{retry && <button onClick={retry}>Retry</button>}</div>;
}
