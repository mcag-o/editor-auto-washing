import { Suspense, lazy, useEffect, useState } from 'react';
import AppShell, { type AppPage } from '../layout/AppShell';
import DashboardPage from '../features/dashboard/DashboardPage';
import IntakePage from '../features/intake/IntakePage';
import ArticlesPage from '../features/articles/ArticlesPage';
import ControlPage from '../features/control/ControlPage';
import ConfigPage from '../features/config/ConfigPage';
import AuditPage from '../features/audit/AuditPage';

const TemplatesPage = lazy(() => import('../features/templates/TemplatesPage'));

const defaultPage: AppPage = 'overview';

function parsePageFromHash(hash: string): AppPage {
  const normalized = hash.replace(/^#/, '');
  if (normalized === '/intake' || normalized === 'intake') {
    return 'intake';
  }
  if (normalized === '/articles' || normalized === 'articles') {
    return 'articles';
  }
  if (normalized === '/overview' || normalized === 'overview') {
    return 'overview';
  }
  if (normalized === '/control' || normalized === 'control') {
    return 'control';
  }
  if (normalized === '/workflows' || normalized === 'workflows') {
    return 'workflows';
  }
  if (normalized === '/audit' || normalized === 'audit') {
    return 'audit';
  }
  if (normalized === '/config' || normalized === 'config') {
    return 'config';
  }
  return defaultPage;
}

export default function AppRoutes() {
  const [currentPage, setCurrentPage] = useState<AppPage>(() => parsePageFromHash(window.location.hash));

  useEffect(() => {
    const handleHashChange = () => {
      setCurrentPage(parsePageFromHash(window.location.hash));
    };

    window.addEventListener('hashchange', handleHashChange);
    return () => window.removeEventListener('hashchange', handleHashChange);
  }, []);

  const handleNavigate = (page: AppPage) => {
    const nextHash = page === 'overview' ? '#/' : `#/${page}`;
    window.location.hash = nextHash;
    setCurrentPage(page);
  };

  return (
    <AppShell currentPage={currentPage} onNavigate={handleNavigate}>
      {currentPage === 'overview' ? <DashboardPage onNavigate={handleNavigate} /> : null}
      {currentPage === 'intake' ? <IntakePage onNavigate={handleNavigate} /> : null}
      {currentPage === 'articles' ? <ArticlesPage onNavigate={handleNavigate} /> : null}
      {currentPage === 'control' ? <ControlPage onNavigate={handleNavigate} /> : null}
      {currentPage === 'workflows' ? (
        <Suspense fallback={null}>
          <TemplatesPage />
        </Suspense>
      ) : null}
      {currentPage === 'audit' ? <AuditPage onNavigate={handleNavigate} /> : null}
      {currentPage === 'config' ? <ConfigPage onNavigate={handleNavigate} /> : null}
    </AppShell>
  );
}
