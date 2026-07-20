import { useEffect, useState } from 'react';
import { useStore } from '../store';

export default function SettingsPage() {
  const [profiles, setProfiles] = useState<any[]>([]);
  const [editing, setEditing] = useState<any>(null);
  const [showForm, setShowForm] = useState(false);
  const [name, setName] = useState('');
  const [provider, setProvider] = useState('');
  const [model, setModel] = useState('');
  const [apiKey, setApiKey] = useState('');
  const [baseUrl, setBaseUrl] = useState('');

  const loadProfiles = async () => {
    try {
      const r = await fetch('/api/v1/llm/profiles');
      setProfiles(await r.json());
    } catch (e) {}
  };

  useEffect(() => { loadProfiles(); }, []);

  const openNew = () => {
    setEditing(null); setName(''); setProvider(''); setModel(''); setApiKey(''); setBaseUrl('');
    setShowForm(true);
  };

  const openEdit = (p: any) => {
    setEditing(p.id); setName(p.name); setProvider(p.provider); setModel(p.model); setApiKey(''); setBaseUrl(p.base_url);
    setShowForm(true);
  };

  const handleSave = async () => {
    const body = { name, provider, model, api_key: apiKey, base_url: baseUrl };
    if (editing) {
      await fetch(`/api/v1/llm/profiles/${editing}`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
    } else {
      await fetch('/api/v1/llm/profiles', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
    }
    setShowForm(false);
    loadProfiles();
  };

  const handleDelete = async (id: string) => {
    await fetch(`/api/v1/llm/profiles/${id}`, { method: 'DELETE' });
    loadProfiles();
  };

  return (
    <div className="card">
      <div className="card-header">
        <h3>LLM Configuration</h3>
        <button className="btn btn-primary" onClick={openNew}>+ Add Profile</button>
      </div>
      {profiles.length === 0 ? (
        <div className="empty-state"><p>No LLM profiles configured yet.</p></div>
      ) : (
        <table>
          <thead><tr><th>Name</th><th>Provider</th><th>Model</th><th>Base URL</th><th>API Key</th><th></th></tr></thead>
          <tbody>
            {profiles.map(p => (
              <tr key={p.id}>
                <td><strong>{p.name}</strong></td>
                <td>{p.provider}</td>
                <td>{p.model}</td>
                <td style={{ fontSize: '0.8rem', color: '#64748b' }}>{p.base_url}</td>
                <td><code>{p.key_hint || '****'}</code></td>
                <td>
                  <button className="btn btn-xs btn-outline" onClick={() => openEdit(p)}>Edit</button>
                  <button className="btn btn-xs btn-danger" style={{ marginLeft: 4 }} onClick={() => handleDelete(p.id)}>Delete</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {showForm && (
        <div className="modal-overlay active" onClick={() => setShowForm(false)}>
          <div className="modal" style={{ maxWidth: 480 }} onClick={e => e.stopPropagation()}>
            <h2>{editing ? 'Edit Profile' : 'New Profile'}</h2>
            <div className="form-group"><label>Name</label><input value={name} placeholder="my-llm" onChange={e => setName(e.target.value)} /></div>
            <div className="form-group"><label>Provider</label><input value={provider} placeholder="openai" onChange={e => setProvider(e.target.value)} /></div>
            <div className="form-group"><label>Model</label><input value={model} placeholder="gpt-4o" onChange={e => setModel(e.target.value)} /></div>
            <div className="form-group"><label>Base URL</label><input value={baseUrl} placeholder="https://api.openai.com" onChange={e => setBaseUrl(e.target.value)} /></div>
            <div className="form-group">
              <label>API Key {editing && <span style={{ color: '#94a3b8', fontSize: '0.75rem' }}>(leave blank to keep current)</span>}</label>
              <input type="password" value={apiKey} placeholder="sk-..." onChange={e => setApiKey(e.target.value)} />
            </div>
            <div className="modal-actions">
              <button className="btn btn-outline" onClick={() => setShowForm(false)}>Cancel</button>
              <button className="btn btn-success" onClick={handleSave}>{editing ? 'Update' : 'Save'}</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}