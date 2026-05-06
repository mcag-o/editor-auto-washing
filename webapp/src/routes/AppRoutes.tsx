import { useEffect, useState } from 'react';
import AppShell, { type AppPage } from '../layout/AppShell';
import DashboardPage from '../features/dashboard/DashboardPage';
import IntakePage from '../features/intake/IntakePage';
import ArticlesPage from '../features/articles/ArticlesPage';
import ControlPage from '../features/control/ControlPage';
import ConfigPage from '../features/config/ConfigPage';
import AuditPage from '../features/audit/AuditPage';

const defaultPage: AppPage = 'dashboard';

function parsePageFromHash(hash: string): AppPage {
  const normalized = hash.replace(/^#/, '');
  if (normalized === '/intake' || normalized === 'intake') {
    return 'intake';
  }
  if (normalized === '/articles' || normalized === 'articles') {
    return 'articles';
  }
  if (normalized === '/workflows' || normalized === 'workflows') {
    return 'workflows';
  }
  if (normalized === '/audit' || normalized === 'audit') {
    return 'audit';
  }
  if (normalized === '/settings' || normalized === 'settings') {
    return 'settings';
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
    const nextHash = page === 'dashboard' ? '#/' : `#/${page}`;
    window.location.hash = nextHash;
    setCurrentPage(page);
  };

  return (
    <AppShell currentPage={currentPage} onNavigate={handleNavigate}>
      {currentPage === 'dashboard' ? <DashboardPage onNavigate={handleNavigate} /> : null}
      {currentPage === 'intake' ? <IntakePage onNavigate={handleNavigate} /> : null}
      {currentPage === 'articles' ? <ArticlesPage onNavigate={handleNavigate} /> : null}
      {currentPage === 'workflows' ? <ControlPage onNavigate={handleNavigate} /> : null}
      {currentPage === 'audit' ? <AuditPage onNavigate={handleNavigate} /> : null}
      {currentPage === 'settings' ? <ConfigPage onNavigate={handleNavigate} /> : null}
    </AppShell>
  );
}
