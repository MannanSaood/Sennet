import { Link } from 'react-router-dom';
import { SennetLogo } from '@/components/brand/SennetLogo';
export function Footer(){return <footer className="docs-footer"><SennetLogo/><p>Connected observability for infrastructure, agents, networks, and financial operations.</p><div><Link to="/docs/architecture">Architecture</Link><Link to="/docs/api">API contracts</Link><Link to="/">Product</Link></div></footer>}
