import { NavLink, Link } from 'react-router-dom';
import { Activity, Layers, Network, Workflow, Landmark, Terminal, Settings, Bell, LayoutDashboard, ScrollText, ChartNoAxesCombined, LogOut } from 'lucide-react';
import { useAuth } from '@/hooks/useAuth';
const links = [
 ['/dashboard','Overview',LayoutDashboard], ['/dashboard/traces','Traces',Layers], ['/dashboard/logs','Logs',ScrollText], ['/dashboard/metrics','Metrics',ChartNoAxesCombined], ['/dashboard/traffic','Network flows',Network], ['/dashboard/map','Service map',Workflow], ['/dashboard/agents','Agent runs',Terminal], ['/dashboard/finance','Finance',Landmark], ['/dashboard/alerts','Monitors',Bell], ['/dashboard/settings','Workspace',Settings],
] as const;
export function DashboardLayout({ children }: { children: React.ReactNode }) {
 const {user,logout}=useAuth();
 return <div className="workspace"><aside className="side-rail"><Link className="brand" to="/"><Activity/>sennet<span className="version-pill">02</span></Link><div className="workspace-label"><span className="status-dot"/><span title={user?.tenant}>{user?.tenant}</span></div><p className="nav-label">INVESTIGATE</p><nav aria-label="Workspace">{links.map(([url,label,Icon])=><NavLink key={url} to={url} end><Icon size={17}/>{label}</NavLink>)}</nav><div className="rail-bottom"><Link to="/docs">Documentation ↗</Link><button onClick={()=>void logout()}><LogOut size={15}/> Sign out</button></div></aside><div className="workspace-main"><header className="workspace-header"><span><span className="muted">Workspace / </span> Investigation</span><span className="role-pill">{user?.role}</span></header><main className="work-content">{children}</main></div></div>;
}
