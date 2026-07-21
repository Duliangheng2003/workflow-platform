import { useEffect, useState, useRef } from 'react';
import { useStore } from '../store';

export default function TemplatesPage() {
  const { templates, loadTemplates, openBuilder, startInstance, deleteTemplate } = useStore();
  useEffect(() => { loadTemplates(); }, []);

  return (
    <div className="card">
      <div className="card-header">
        <h3>Workflow Templates</h3>
        <button className="btn btn-primary" onClick={() => openBuilder()}>+ New Template</button>
      </div>
      {templates.length === 0 ? (
        <div className="empty-state"><p>No templates yet.</p></div>
      ) : (
        <div className="template-grid">
          {templates.map(t => (
            <TemplateCard key={t.id} t={t} onEdit={() => openBuilder(t)} onRun={() => startInstance(t.id)} onDelete={() => deleteTemplate(t.id)} />
          ))}
        </div>
      )}
    </div>
  );
}

function TemplateCard({ t, onEdit, onRun, onDelete }: any) {
  const [open, setOpen] = useState(false);
  const cardRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (cardRef.current && !cardRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [open]);

  const handleClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    setOpen(!open);
  };

  const st = t.start_type || 'User Input';
  return (
    <div className="tmpl-card" ref={cardRef} style={open ? { zIndex: 9999 } : undefined}>
      <div className="tmpl-card-top">
        <div className="tmpl-card-name">{t.name}</div>
        <div className={'tmpl-card-menu-wrap' + (open ? ' open' : '')}>
          <button className="tmpl-card-menu-btn" onClick={handleClick}>⋮</button>
          {open && (
            <div className="tmpl-card-menu active" onClick={e => e.stopPropagation()}>
              <div className="tmpl-menu-item" onClick={() => { setOpen(false); onRun(); }}>▶ Run</div>
              <div className="tmpl-menu-item" onClick={() => { setOpen(false); onEdit(); }}>✎ Edit</div>
              <div className="tmpl-menu-divider" />
              <div className="tmpl-menu-item tmpl-menu-item-danger" onClick={() => { setOpen(false); onDelete(); }}>✕ Delete</div>
            </div>
          )}
        </div>
      </div>
      <div className="tmpl-card-bottom">
        {t.description && <div className="tmpl-card-desc">{t.description}</div>}
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