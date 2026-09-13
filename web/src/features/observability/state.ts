import { useCallback, useMemo } from 'react';
import { useSearchParams } from 'react-router-dom';

export const ranges = [
  ['900000', 'Last 15 minutes'], ['3600000', 'Last hour'], ['86400000', 'Last 24 hours'],
  ['604800000', 'Last 7 days'], ['2592000000', 'Last 30 days'],
] as const;

export function useInvestigationState() {
  const [params, setParams] = useSearchParams();
  const state = useMemo(() => ({
    range: params.get('range') || '3600000',
    timezone: params.get('timezone') || Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
    environment: params.get('environment') || '', service: params.get('service') || '',
    tenant: params.get('tenant') || '', search: params.get('q') || '', trace: params.get('trace_id') || '',
    cursor: params.get('cursor') || '', paused: params.get('paused') === 'true',
  }), [params]);
  const update = useCallback((updates: Record<string, string>) => setParams(previous => {
    const next = new URLSearchParams(previous);
    Object.entries(updates).forEach(([key, value]) => value ? next.set(key, value) : next.delete(key));
    if (!Object.hasOwn(updates, 'cursor')) next.delete('cursor');
    return next;
  }, { replace: true }), [setParams]);
  return { state, update, params };
}

export function formatTime(time: number, timezone: string, withDate = false) {
  try { return new Intl.DateTimeFormat(undefined, { timeZone: timezone, ...(withDate ? { dateStyle: 'medium', timeStyle: 'medium' } : { hour: '2-digit', minute: '2-digit', second: '2-digit' }) }).format(time); }
  catch { return new Date(time).toISOString(); }
}
