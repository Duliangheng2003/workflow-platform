import { useEffect, useState } from 'react';
import { useStore } from '../store';
import type { Template } from '../types';

export default function TemplatesPage() {
  const { templates, loadTemplates, openBuilder, startInstance, deleteTemplate } = useStore();
  const [openMenu, setOpenMenu] = useState<string | null>(null);
  useEffect(() => { loadTemplates(); }, []);

  return (
    <div className="card" onClick={() => setOpenMenu(null)}>
      <div className="card-header">
        <h3>Workflow Templates</h3>
        <button className="btn btn-primary" onClick={() => openBuilder()}>+ New Template</button>
      </div>
      {templates.length === 0 ? (
        <div className="empty-state"><p>No templates yet.</p></div>
      ) : (
        <div className="template-grid">
          {templates.map(t => (
            <TemplateCard
              key={t.id}
              t={t}
              menuOpen={openMenu === t.id}
              onMenuToggle={(e) => { e.stopPropagation(); setOpenMenu(openMenu === t.id ? null : t.id); }}
              onEdit={() => { setOpenMenu(null); openBuilder(t); }}
              onRun={() => { setOpenMenu(null); startInstance(t.id); }}
              onDelete={() => { setOpenMenu(null); deleteTemplate(t.id); }}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function TemplateCard({ t, menuOpen, onMenuToggle, onEdit, onRun, onDelete }: any) {
  const st = t.start_type || 'User Input';
  return (
    <div className="tmpl-card">
      <div className="tmpl-card-top">
        <div className="tmpl-card-name">{t.name}</div>
        <div className="tmpl-card-menu-btn-wrap">
          <button className="tmpl-card-menu-btn" onClick={onMenuToggle}>⋮</button>
          {menuOpen && (
            <div className="tmpl-card-menu active" style={{ position: 'fixed', zIndex: 9999 }} onClick={e => e.stopPropagation()}>
              <div className="tmpl-menu-item" onClick={onRun}>▶ Run</div>
              <div className="tmpl-menu-item" onClick={onEdit}>✎ Edit</div>
              <div className="tmpl-menu-divider" />
              <div className="tmpl-menu-item tmpl-menu-item-danger" onClick={onDelete}>✕ Delete</div>
            </div>
          )}
        </div>
      </div>
      <div className="tmpl-card-bottom">
        <div className="tmpl-card-meta">
          <span className={`tmpl-badge ${st === 'Schedule' ? 'tmpl-badge-schedule' : 'tmpl-badge-manual'}`}>{st}</span>
          <span className="tmpl-meta-item">{(t.nodes || []).length} nodes</span>
          <span className="tmpl-meta-item">{fmtTime(t.created_at)}</span>
        </div>
      </div>
    </div>
  );
}

function fmtTime(t: string) {
  if (!t) return '-';
  const d = new Date(t);
  return `${d.getFullYear()}/${pad(d.getMonth()+1)}/${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}
function pad(n: number) { return n < 10 ? '0' + n : '' + n; }