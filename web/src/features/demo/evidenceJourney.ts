export type EvidenceStage = {
  id: string;
  lane: 'edge' | 'agent' | 'service' | 'network' | 'finance';
  label: string;
  service: string;
  startMs: number;
  durationMs: number;
  status: 'ok' | 'waiting' | 'error' | 'recovered';
  evidence: string;
};

export const DEMO_TRACE_ID = 'demo-7f34b91a';

export const evidenceJourney: EvidenceStage[] = [
  { id:'request', lane:'edge', label:'User request', service:'checkout-web', startMs:0, durationMs:18, status:'ok', evidence:'POST /orders · request accepted' },
  { id:'gateway', lane:'service', label:'Gateway', service:'edge-gateway', startMs:11, durationMs:42, status:'ok', evidence:'Tenant and schema validated' },
  { id:'supervisor', lane:'agent', label:'Agent supervisor', service:'order-supervisor', startMs:29, durationMs:268, status:'recovered', evidence:'Run opened · 2 parallel handoffs' },
  { id:'risk-agent', lane:'agent', label:'Risk subagent', service:'risk-agent', startMs:48, durationMs:126, status:'ok', evidence:'Evaluation 0.92 · policy set v18' },
  { id:'route-agent', lane:'agent', label:'Route subagent', service:'route-agent', startMs:52, durationMs:156, status:'recovered', evidence:'Provider timeout · retry succeeded' },
  { id:'model', lane:'agent', label:'Model call', service:'model-router', startMs:64, durationMs:71, status:'ok', evidence:'1,284 input · 196 output tokens' },
  { id:'tool', lane:'agent', label:'Ledger lookup', service:'ledger-tool', startMs:91, durationMs:38, status:'ok', evidence:'Tool result linked to parent span' },
  { id:'payments', lane:'service', label:'Application services', service:'payment-api', startMs:174, durationMs:81, status:'ok', evidence:'Authorization requested · USD 1,250.00' },
  { id:'network', lane:'network', label:'Network transmission', service:'payments → acquirer', startMs:187, durationMs:47, status:'waiting', evidence:'31 ms queue · 14 ms transit · TCP' },
  { id:'risk', lane:'finance', label:'Risk check', service:'risk-engine', startMs:211, durationMs:28, status:'ok', evidence:'Observed decision: allow' },
  { id:'settlement', lane:'finance', label:'Settlement', service:'settlement-worker', startMs:255, durationMs:122, status:'recovered', evidence:'Capture → settlement · one corrected sequence gap' },
  { id:'result', lane:'edge', label:'Final result', service:'checkout-web', startMs:377, durationMs:15, status:'ok', evidence:'Order confirmed · end-to-end 392 ms' },
];

export const demoLanes = ['edge','agent','service','network','finance'] as const;
