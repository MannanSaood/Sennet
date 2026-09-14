import { Navbar } from '@/components/layout/Navbar';
import { DocsSidebar } from '@/components/layout/DocsSidebar';
import { Footer } from '@/components/layout/Footer';
export function DocsLayout({children,toc}:{children:React.ReactNode;toc?:{id:string;label:string}[]}){return <div className="docs-shell"><Navbar/><div className="docs-body"><DocsSidebar/><main className="docs-main">{children}</main><aside className="docs-toc" aria-label="On this page"><span>ON THIS PAGE</span>{toc?.map(item=><a key={item.id} href={`#${item.id}`}>{item.label}</a>)}<a href="#top">Back to top ↑</a></aside></div><Footer/></div>}
