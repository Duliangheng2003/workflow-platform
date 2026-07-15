import { useStore } from './store';
import TemplatesPage from './pages/TemplatesPage';
import InstancesPage from './pages/InstancesPage';
import BuilderPage from './pages/BuilderPage';
import Toast from './components/Toast';

export default function App() {
  const { page, setPage } = useStore();
  return (
    <>
      <header>
        <h1>Workflow Platform</h1>
        <nav>
          <a className={page === 'templates' ? 'active' : ''} onClick={() => setPage('templates')}>Templates</a>
          <a className={page === 'instances' ? 'active' : ''} onClick={() => setPage('instances')}>Instances</a>
        </nav>
      </header>
      <main>
        {page === 'templates' && <TemplatesPage />}
        {page === 'instances' && <InstancesPage />}
      </main>
      {page === 'builder' && <BuilderPage />}
      <Toast />
    </>
  );
}