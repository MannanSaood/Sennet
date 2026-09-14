import { useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { Menu, X } from 'lucide-react';
import { SennetLogo } from '@/components/brand/SennetLogo';

export function Navbar(){const [open,setOpen]=useState(false);const location=useLocation();return <header className="docs-nav"><SennetLogo/><button className="docs-menu" aria-expanded={open} aria-label="Toggle navigation" onClick={()=>setOpen(value=>!value)}>{open?<X/>:<Menu/>}</button><nav className={open?'is-open':''} aria-label="Documentation header"><Link to="/">Product</Link><Link to="/docs">Documentation</Link><Link to="/docs/quickstart">Quickstart</Link><Link to="/login" className="button button--small" onClick={()=>setOpen(false)}>Open workspace</Link></nav><span className="docs-location">{location.pathname.replace('/docs/','').replace('/docs','Overview')}</span></header>}
