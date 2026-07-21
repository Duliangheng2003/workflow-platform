import { useEffect, useState } from 'react';
import { useStore } from '../store';
import { instancesApi } from '../api';
import type { Instance } from '../types';
import { marked } from 'marked';

export default function InstancesPage() {
  const { instances, loadInstances, deleteInstance, templates } = useStore();
  const [detail, setDetail] = useState<Instance | null>(null);
  const [expanded, setExpanded] = useState<string | null>(null);
  useEffect(() => {
    loadInstances();
    const hasRunning = instances?.some(i => i.status === 'running' || i.status === 'pending');
    if (!hasRunning) return;
    const t = setInterval(loadInstances, 3000);
    return () => clearInterval(t);
  }, []);

  const tmplMap: Record<string, string> = {};
  templates.forEach(t => { tmplMap[t.id] = t.name; });

  const showDetail = async (id: string) => {
    const inst = await instancesApi.get(id);
    setDetail(inst);
    setExpanded(null);
  };

  return (
    <div className="card">
      <div className="card-header"><h3>Instances</h3></div>
      {instances?.length === 0 ? (
        <div className="empty-state"><p>No instances yet.</p></div>
      ) : (
        <table>
          <thead><tr><th>ID</th><th>Template</th><th>Status</th><th>Time</th><th></th></tr></thead>
          <tbody>
            {instances.map(i => (
              <tr key={i.id}>
                <td><code>{i.id.substring(0, 10)}...</code></td>
                <td>{tmplMap[i.template_id] || i.template_id}</td>
                <td><span className={`badge badge-${i.status}`}>{i.status}</span></td>
                <td>{fmtTime(i.created_at)}</td>
                <td>
                  <button className="btn btn-xs btn-outline" onClick={() => showDetail(i.id)}>Detail</button>
                  <button className="btn btn-xs btn-outline btn-del" style={{ marginLeft: 4 }} onClick={() => deleteInstance(i.id)}>Delete</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {detail && (
        <div className="modal-overlay active" onClick={() => setDetail(null)}>
          <div className="modal" style={{ maxWidth: 700 }} onClick={e => e.stopPropagation()}>
            <h2>Instance Detail</h2>
            <div className="detail-grid">
              <div><div className="detail-label">ID</div><code>{detail.id}</code></div>
              <div><div className="detail-label">Status</div><span className={`badge badge-${detail.status}`}>{detail.status}</span></div>
              <div><div className="detail-label">Template</div>{tmplMap[detail.template_id] || detail.template_id}</div>
              <div><div className="detail-label">Current Node</div>{detail.current_node_id || '-'}</div>
            </div>
            {detail.error && <div className="error-box">{detail.error}</div>}
            {detail.node_states && Object.keys(detail.node_states).length > 0 && (
              <div style={{ marginTop: 12 }}>
                <div className="detail-label">Node States</div>
                <div className="node-states-list">
                  {Object.keys(detail.node_states).map(nid => {
                    const ns = detail.node_states[nid];
                    const isOpen = expanded === nid;
                    const nodeData = detail.state?.[nid];
                    return (
                      <div key={nid} className="nc-card">
                        <div className="nc-header" onClick={() => setExpanded(isOpen ? null : nid)}>
                          <span className="nc-arrow">{isOpen ? '▼' : '▶'}</span>
                          <code className="nc-name">{nid}</code>
                          <span className={`ns-badge ns-badge-${ns.status}`}>{ns.status}</span>
                          {ns.error && <span className="nc-err">{ns.error}</span>}
                        </div>
                        {isOpen && (
                          <div className="nc-body">
                            {ns.error && <div className="nc-error">{ns.error}</div>}
                            {nodeData && (
                              <>
                                {nodeData.output && (
                                  <>
                                    <div className="nc-label">Output</div>
                                    <div className="json-box">{formatOutput(nodeData.output)}</div>
                                  </>
                                )}
                                {nodeData.content && (
                                  <>
                                    <div className="nc-label">Response</div>
                                    <div className="md-content" dangerouslySetInnerHTML={{ __html: marked.parse(nodeData.content) }} />
                                  </>
                                )}
                                {nodeData.stderr && (
                                  <>
                                    <div className="nc-label" style={{ color: '#dc2626' }}>stderr</div>
                                    <div style={{ fontSize: '0.78rem', color: '#dc2626', fontFamily: 'monospace' }}>{nodeData.stderr}</div>
                                  </>
                                )}
                                {nodeData.result !== undefined && (
                                  <>
                                    <div className="nc-label">Result</div>
                                    <div style={{ fontSize: '0.82rem' }}>{nodeData.result ? 'true' : 'false'}{nodeData.port && <span style={{ color: '#64748b', marginLeft: 8 }}>port: {nodeData.port}</span>}</div>
                                  </>
                                )}
                              </>
                            )}
                          </div>
                        )}
                      </div>
                    );
                  })}
                </div>
              </div>
            )}
            <div className="modal-actions"><button className="btn btn-outline" onClick={() => setDetail(null)}>Close</button></div>
          </div>
        </div>
      )}
    </div>
  );
}

function formatOutput(v: any) {
  if (typeof v !== 'string') return JSON.stringify(v, null, 2);
  try { return JSON.stringify(JSON.parse(v), null, 2); } catch (e) { return v; }
}

function fmtTime(t: string) {
  if (!t) return '-';
  const d = new Date(t);
  return `${d.getFullYear()}/${pad(d.getMonth()+1)}/${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}
function pad(n: number) { return n < 10 ? '0' + n : '' + n; }