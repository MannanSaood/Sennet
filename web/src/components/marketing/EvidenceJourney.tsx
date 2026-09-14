import { useEffect, useRef, useState } from 'react';
import { evidenceJourney, DEMO_TRACE_ID, demoLanes } from '@/features/demo/evidenceJourney';

const laneNames = { edge:'Request', agent:'Agents', service:'Services', network:'Network', finance:'Finance' };

export function EvidenceJourney() {
  const [active, setActive] = useState(0);
  const stepRefs = useRef<(HTMLElement | null)[]>([]);
  useEffect(() => {
    const observer = new IntersectionObserver(entries => {
      const visible = entries.filter(entry => entry.isIntersecting).sort((a,b)=>b.intersectionRatio-a.intersectionRatio)[0];
      if (visible) setActive(Number((visible.target as HTMLElement).dataset.step));
    }, { rootMargin:'-36% 0px -42%', threshold:[0,.5,1] });
    stepRefs.current.forEach(step => step && observer.observe(step));
    return () => observer.disconnect();
  }, []);
  const current = evidenceJourney[active];
  return <section className="journey" id="journey" aria-labelledby="journey-title">
    <div className="journey__intro"><span className="section-index">01 / FOLLOW</span><h2 id="journey-title">One request.<br/>Every consequence.</h2><p>The same deterministic trace moves through infrastructure, autonomous decisions, network transit, and a financial lifecycle. This is a labelled product demonstration—not live customer telemetry.</p></div>
    <div className="journey__layout"><div className="journey__viewport" aria-live="polite"><div className="demo-label"><span/> DEMO DATASET <code>{DEMO_TRACE_ID}</code></div><div className="trace-ruler"><span>0 ms</span><span>100</span><span>200</span><span>300</span><span>392 ms</span></div><div className="trace-stage">
      {demoLanes.map(lane => <div className="trace-lane" key={lane}><strong>{laneNames[lane]}</strong><div>{evidenceJourney.filter(item=>item.lane===lane).map((item,index)=>{const originalIndex=evidenceJourney.indexOf(item);return <button key={item.id} className={`trace-clip is-${item.status} ${originalIndex===active?'is-active':''}`} style={{left:`${item.startMs/4.2}%`,width:`${Math.max(7,item.durationMs/4.2)}%`}} onClick={()=>setActive(originalIndex)} aria-label={`${item.label}, ${item.durationMs} milliseconds`}><i style={{animationDelay:`${index*70}ms`}}/>{item.label}</button>})}</div></div>)}
      <div className="trace-playhead" style={{left:`calc(118px + (100% - 118px) * ${Math.min(1,(current.startMs+current.durationMs)/420)})`}} aria-hidden/></div><div className="journey__inspector"><div><span>FOCUS</span><strong>{current.label}</strong></div><div><span>SOURCE</span><strong>{current.service}</strong></div><div><span>DURATION</span><strong>{current.durationMs} ms</strong></div><div><span>EVIDENCE</span><strong>{current.evidence}</strong></div></div></div>
      <div className="journey__steps">{evidenceJourney.map((stage,index)=><article key={stage.id} data-step={index} ref={node=>{stepRefs.current[index]=node}} className={index===active?'is-active':''}><span>{String(index+1).padStart(2,'0')}</span><div><h3>{stage.label}</h3><p>{stage.evidence}</p></div></article>)}</div></div>
  </section>;
}
