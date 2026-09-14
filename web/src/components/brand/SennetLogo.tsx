import { Link } from 'react-router-dom';

type MarkProps = { className?: string; title?: string; mono?: boolean };

export function SennetMark({ className, title = 'Sennet', mono = false }: MarkProps) {
  return <svg className={className} viewBox="0 0 48 48" role={title ? 'img' : undefined} aria-hidden={title ? undefined : true}>
    {title ? <title>{title}</title> : null}
    <path d="M37 8H20.5C14.7 8 10 12.7 10 18.5S14.7 29 20.5 29h7C31.1 29 34 31.9 34 35.5S31.1 42 27.5 42H11" fill="none" stroke="currentColor" strokeWidth="5" strokeLinecap="round"/>
    <circle cx="37" cy="8" r="4" fill={mono ? 'currentColor' : 'var(--signal-amber, #ffb547)'}/>
    <circle cx="10" cy="18.5" r="2.6" fill="currentColor"/>
    <circle cx="34" cy="35.5" r="2.6" fill="currentColor"/>
    <circle cx="11" cy="42" r="4" fill={mono ? 'currentColor' : 'var(--signal-blue, #7dcfff)'}/>
  </svg>;
}

export function SennetLogo({ compact = false, to = '/', className = '' }: { compact?: boolean; to?: string; className?: string }) {
  return <Link to={to} className={`sennet-logo ${compact ? 'is-compact' : ''} ${className}`} aria-label="Sennet home">
    <SennetMark className="sennet-logo__mark" title="" />
    {compact ? null : <span className="sennet-logo__word">SENNET</span>}
  </Link>;
}
