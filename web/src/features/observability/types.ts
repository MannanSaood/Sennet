export interface TelemetryEvent { id: string; time: number; signal: string; service: string; name: string; trace_id?: string; span_id?: string; parent_id?: string; duration_ms: number; status: string; value: number; attributes: Record<string,string> }
export interface EventPage { events: TelemetryEvent[]; next_cursor: string; timestamp: number; partial: boolean }
export interface Agent { id: string; version: string; seen: number; collection: string; metrics: Record<string,string> }
export const signalNames: Record<string,string> = {trace:'Traces',log:'Logs',metric:'Metrics',flow:'Network flows',agent:'Agent runs',finance:'Financial events'};
export function formatNumber(n: number) {return new Intl.NumberFormat('en',{notation:'compact',maximumFractionDigits:1}).format(n);}
