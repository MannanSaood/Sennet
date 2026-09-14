export type DocEntry={slug:string;title:string;section:string;description:string;video?:{poster:string;mp4:string;webm:string;captions:string;duration:string;chapters:{time:string;label:string}[];ready:boolean}}
export const docsNavigation:DocEntry[]=[
 {slug:'introduction',title:'Sennet overview',section:'Start',description:'What Sennet observes and the boundaries it keeps.'},
 {slug:'overview-video',title:'Two-minute overview',section:'Start',description:'A compact product tour with transcript and chapter metadata.',video:{poster:'/brand/sennet-og-lockup.svg',mp4:'/media/docs/sennet-two-minute-overview.mp4',webm:'/media/docs/sennet-two-minute-overview.webm',captions:'/media/docs/sennet-two-minute-overview.en.vtt',duration:'02:00',ready:false,chapters:[{time:'00:00',label:'One connected investigation'},{time:'00:34',label:'Follow a distributed trace'},{time:'01:08',label:'Agent and financial evidence'},{time:'01:40',label:'Build a dashboard'}]}},
 {slug:'quickstart',title:'Ingest the first signal',section:'Start',description:'Run locally and send real or explicit evaluation telemetry.'},
 {slug:'installation',title:'Installation',section:'Start',description:'Local and streaming evaluation paths.'},
 {slug:'infrastructure',title:'Infrastructure walkthrough',section:'Investigate',description:'Explore services, traces, metrics, logs, and network evidence.'},
 {slug:'distributed-trace',title:'Distributed trace',section:'Investigate',description:'Move from an error to its bounded trace and dependencies.'},
 {slug:'agent-runs',title:'Multi-agent runs',section:'Investigate',description:'Read handoffs, retries, cost, and critical paths.'},
 {slug:'financial-settlement',title:'Financial settlement',section:'Investigate',description:'Inspect exact values, sequence gaps, and event-time delay.'},
 {slug:'dashboards',title:'Build a dashboard',section:'Operate',description:'Compose, version, and persist synchronized panels.'},
 {slug:'monitors',title:'Create a monitor',section:'Operate',description:'Persist threshold and SLO definitions.'},
 {slug:'metrics',title:'Metrics and querying',section:'Operate',description:'Aggregation, distributions, rates, and limits.'},
 {slug:'architecture',title:'Architecture',section:'Reference',description:'Durability, trust, storage, and query boundaries.'},
 {slug:'configuration',title:'Configuration',section:'Reference',description:'Identity, deployment, migration, and recovery.'},
 {slug:'ebpf',title:'eBPF collection',section:'Reference',description:'Linux collection capabilities and limits.'},
 {slug:'cli',title:'Agent commands',section:'Reference',description:'Collector and local investigation commands.'},
 {slug:'api',title:'API contracts',section:'Reference',description:'Authenticated endpoints and event schema.'},
];
