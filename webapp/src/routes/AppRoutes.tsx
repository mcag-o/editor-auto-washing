import { useEffect, useState } from 'react';
import AppShell, { type AppPage } from '../layout/AppShell';
import PlaceholderPage from '../features/placeholder/PlaceholderPage';
import DashboardPage from '../features/dashboard/DashboardPage';
import IntakePage from '../features/intake/IntakePage';
import ArticlesPage from '../features/articles/ArticlesPage';

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
      {currentPage === 'workflows' ? (
        <PlaceholderPage
          title="流程模板"
          moduleName="流程模板"
          description="流程模板入口已经保留在外壳导航中，本里程碑仅提供稳定占位页，不开始 Task 4 的真实实现。"
          onNavigate={handleNavigate}
        />
      ) : null}
      {currentPage === 'audit' ? (
        <PlaceholderPage
          title="操作审计"
          moduleName="操作审计"
          description="操作审计入口在当前里程碑中仅作为页面壳层占位，确保导航有确定落点。"
          onNavigate={handleNavigate}
        />
      ) : null}
      {currentPage === 'settings' ? (
        <PlaceholderPage
          title="系统配置"
          moduleName="系统配置"
          description="系统配置已纳入整体信息架构，但本次只保留占位页，后续任务再接入真实配置界面。"
          onNavigate={handleNavigate}
        />
      ) : null}
    </AppShell>
  );
}
