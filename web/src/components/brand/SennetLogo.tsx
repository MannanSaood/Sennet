import { Link } from 'react-router-dom';

type MarkProps = { className?: string; title?: string; mono?: boolean };

export function SennetMark({ className, title = 'Sennet', mono = false }: MarkProps) {
  return <svg className={className} viewBox="0 0 64 64" role={title ? 'img' : undefined} aria-hidden={title ? undefined : true}>
    {title ? <title>{title}</title> : null}
    <path d="M7 31C5 18 13 7 27 4c7-1.5 14-.5 20 3L36 17c-5.5 4.7-7.5 8.5-7.5 14H7Z" fill="currentColor" opacity={mono ? 1 : .34}/>
    <path d="M57 33c2 13-6 24-20 27-7 1.5-14 .5-20-3l11-10c5.5-4.7 7.5-8.5 7.5-14H57Z" fill={mono ? 'currentColor' : 'var(--signal-blue, #2c789d)'}/>
    <path d="M13 44c8 0 11-5 16-10s2-10 8-15c4-3 8-4 14-3" fill="none" stroke="currentColor" strokeWidth="4.5" strokeLinecap="round"/>
    <circle cx="13" cy="44" r="5.5" fill={mono ? 'currentColor' : 'var(--signal-blue, #2c789d)'}/>
    <circle cx="51" cy="16" r="5.5" fill={mono ? 'currentColor' : 'var(--signal-amber, #ffb547)'}/>
  </svg>;
}

export function SennetLogo({ compact = false, to = '/', className = '' }: { compact?: boolean; to?: string; className?: string }) {
  return <Link to={to} className={`sennet-logo ${compact ? 'is-compact' : ''} ${className}`} aria-label="Sennet home">
    <SennetMark className="sennet-logo__mark" title="" />
    {compact ? null : <span className="sennet-logo__word">SENNET</span>}
  </Link>;
}
