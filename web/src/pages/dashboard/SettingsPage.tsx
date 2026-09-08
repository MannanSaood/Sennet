import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { api } from '@/api/client';
import { useAuth } from '@/hooks/useAuth';
import { DashboardLayout } from '@/components/dashboard/DashboardLayout';

interface View {
  id: string;
  name: string;
  path: string;
  range: string;
  signal: string;
  search: string;
  service: string;
}

export function SettingsPage() {
  const { user } = useAuth();
  const views = useQuery({
    queryKey: ['views', user?.tenant],
    queryFn: async () => (await api.get<View[]>('/api/dashboards')).data,
  });

  return (
    <DashboardLayout>
      <div className="page-heading">
        <div>
          <p className="eyebrow">WORKSPACE</p>
          <h1>Account & saved views</h1>
          <p className="muted">Tenant: {user?.tenant}. Access follows your signed-in account; there are no API keys to create or manage.</p>
        </div>
      </div>
      <section className="panel">
        <div className="panel-heading"><h2>Signed-in access</h2><span>{user?.role ?? 'member'}</span></div>
        <p>Your short-lived login session authenticates dashboard, query, and ingestion requests automatically.</p>
      </section>
      <section className="panel">
        <div className="panel-heading"><h2>Saved investigation views</h2></div>
        {views.isError ? <p className="error-banner">Saved views unavailable.</p> : views.data?.length ? (
          <div className="saved-grid">
            {views.data.map((view) => (
              <Link key={view.id} to={`${view.path.startsWith('/dashboard') ? view.path : '/dashboard'}?${new URLSearchParams({ range: view.range || '3600000', search: view.search || '', service: view.service || '' })}`}>
                {view.name} →
              </Link>
            ))}
          </div>
        ) : <p className="empty">Use “Save view” from any explorer to return to an investigation.</p>}
      </section>
      <p className="muted">Cloud billing connectors and shared-team administration are not enabled on this deployment. There are no simulated billing or notification settings.</p>
    </DashboardLayout>
  );
}
