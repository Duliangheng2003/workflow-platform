import { useState, useEffect, useRef, useCallback } from 'react';
import { useStore, NODE_COLORS } from '../store';
import type { Node } from '../types';

const GAP = 220, GAPY = 100;

export default function BuilderPage() {
  const s = useStore();
  const { builderNodes, builderEdges, selectedNodeId, nodePositions, startType, startInput,
    closeBuilder, setSelectedNode, addNode, deleteNode, addEdge, updateNode, updateAgent,
    undo, redo, setStartType, saveWorkflow, editingTemplateId, templates } = s;
  const [saveOpen, setSaveOpen] = useState(false);
  const [saveName, setSaveName] = useState('');
  const [saveDesc, setSaveDesc] = useState('');
  const [addPopup, setAddPopup] = useState(false);
  const [panelCollapsed, setPanelCollapsed] = useState(false);
  const [panelWidth, setPanelWidth] = useState(280);
  const [connectFrom, setConnectFrom] = useState<string | null>(null);
  const [connectType, setConnectType] = useState('flow');
  const [connectPort, setConnectPort] = useState('');
  const [connectPos, setConnectPos] = useState({ x: 0, y: 0 });
  const [ghostNode, setGhostNode] = useState<any>(null);
  const [ghostPos, setGhostPos] = useState({ x: 0, y: 0 });
  const canvasRef = useRef<HTMLDivElement>(null);
  const [drag, setDrag] = useState<any>(null);
  const [canvasScroll, setCanvasScroll] = useState({ x: 0, y: 0 });

  const calcLayout = useCallback(() => {
    const children: Record<string, string[]> = { '_start': [] };
    builderNodes.forEach(n => { children[n.id] = []; });
    builderEdges.forEach(e => { if (e.edge_type !== 'data' && children[e.from]) children[e.from].push(e.to); });
    const levels: Record<string, number> = {}, queue: { id: string; depth: number }[] = [];
    const visited = new Set<string>();
    (children['_start'] || []).forEach(c => queue.push({ id: c, depth: 0 }));
    const hasIncoming: Record<string, boolean> = {};
    builderEdges.forEach(e => { if (e.edge_type !== 'data') hasIncoming[e.to] = true; });
    builderNodes.forEach(n => { if (!hasIncoming[n.id] && !(n.id in levels)) queue.push({ id: n.id, depth: 0 }); });
    while (queue.length) {
      const cur = queue.shift()!;
      if (visited.has(cur.id)) continue;
      visited.add(cur.id); levels[cur.id] = cur.depth;
      (children[cur.id] || []).forEach(c => { if (!visited.has(c)) queue.push({ id: c, depth: cur.depth + 1 }); });
    }
    const byLevel: Record<number, string[]> = {};
    Object.keys(levels).forEach(id => { const lv = levels[id]; if (!byLevel[lv]) byLevel[lv] = []; byLevel[lv].push(id); });
    const pos: Record<string, { x: number; y: number }> = { _start: nodePositions['_start'] || { x: 60, y: 260 } };
    Object.keys(byLevel).forEach(lv => {
      const ids = byLevel[parseInt(lv)], startY = 260 - (ids.length - 1) * GAPY / 2;
      ids.forEach((id, i) => { pos[id] = nodePositions[id] || { x: (parseInt(lv) + 1) * GAP + 60, y: startY + i * GAPY }; });
    });
    return pos;
  }, [builderNodes, builderEdges, nodePositions]);

  const pos = calcLayout();

  // Canvas mouse handlers
  const handleCanvasMouseDown = (e: React.MouseEvent) => {
    if (e.target === canvasRef.current || (e.target as HTMLElement).closest('.canvas-viewport')) {
      setDrag({ type: 'pan', sx: e.clientX, sy: e.clientY, sl: canvasScroll.x, st: canvasScroll.y });
    }
  };

  const handleMouseMove = (e: MouseEvent) => {
    if (ghostNode) {
      if (!canvasRef.current) return;
      const cr = canvasRef.current.getBoundingClientRect();
      setGhostPos({ x: e.clientX - cr.left + canvasScroll.x - 86, y: e.clientY - cr.top + canvasScroll.y - 20 });
      return;
    }
    if (connectFrom) {
      if (!canvasRef.current) return;
      const cr = canvasRef.current.getBoundingClientRect();
      setConnectPos({ x: e.clientX - cr.left + canvasScroll.x, y: e.clientY - cr.top + canvasScroll.y });
      return;
    }
    if (!drag) return;
    if (drag.type === 'pan') {
      setCanvasScroll({ x: drag.sl - (e.clientX - drag.sx), y: drag.st - (e.clientY - drag.sy) });
    }
    if (drag.type === 'node') {
      if (!canvasRef.current) return;
      const cr = canvasRef.current.getBoundingClientRect();
      const x = e.clientX - cr.left - drag.ox + canvasScroll.x;
      const y = e.clientY - cr.top - drag.oy + canvasScroll.y;
      useStore.setState({ nodePositions: { ...useStore.getState().nodePositions, [drag.id]: { x, y } } });
    }
  };

  const handleMouseUp = (e: MouseEvent) => {
    if (ghostNode) {
      if (!canvasRef.current) return;
      const cr = canvasRef.current.getBoundingClientRect();
      const x = e.clientX - cr.left + canvasScroll.x - 86;
      const y = e.clientY - cr.top + canvasScroll.y - 20;
      useStore.setState({
        builderNodes: [...builderNodes, ghostNode],
        nodePositions: { ...useStore.getState().nodePositions, [ghostNode.id]: { x, y } },
        selectedNodeId: ghostNode.id,
      });
      setGhostNode(null);
      setDrag(null);
      return;
    }
    if (connectFrom) {
      const target = e.target as HTMLElement;
      const isData = connectType === 'data';
      const targetClass = isData ? '.node-port.data' : '.node-port.input';
      const portEl = target.closest(targetClass);
      const nodeEl = target.closest('.node');
      if (portEl && nodeEl) {
        const id = nodeEl.getAttribute('data-id');
        if (id && id !== connectFrom && id !== '_start') {
          addEdge(connectFrom, id, connectType, connectPort);
        }
      }
      setConnectFrom(null);
      setConnectType('flow');
      setConnectPort('');
      setDrag(null);
      return;
    }
    setDrag(null);
  };

  useEffect(() => {
    document.addEventListener('mousemove', handleMouseMove);
    document.addEventListener('mouseup', handleMouseUp);
    return () => {
      document.removeEventListener('mousemove', handleMouseMove);
      document.removeEventListener('mouseup', handleMouseUp);
    };
  }, [drag, ghostNode, connectFrom, builderNodes, canvasScroll, connectType, connectPort, nodePositions]);

  const handleNodeMouseDown = (e: React.MouseEvent, nodeId: string) => {
    if ((e.target as HTMLElement).closest('.node-port')) return;
    e.stopPropagation();
    const el = (e.target as HTMLElement).closest('.node') as HTMLElement;
    if (!el) return;
    const rect = el.getBoundingClientRect();
    setDrag({ type: 'node', id: nodeId, ox: e.clientX - rect.left, oy: e.clientY - rect.top });
    setSelectedNode(nodeId);
  };

  const handleNodeClick = (nodeId: string) => {
    setSelectedNode(nodeId);
  };

  const handleSave = async () => {
    if (!saveName) return;
    await saveWorkflow(saveName, saveDesc);
    setSaveOpen(false);
  };

  const openSave = () => {
    if (editingTemplateId) {
      const t = templates.find(x => x.id === editingTemplateId);
      setSaveName(t?.name || ''); setSaveDesc(t?.description || '');
    } else { setSaveName(''); setSaveDesc(''); }
    setSaveOpen(true);
  };

  const nodeTypes = ['call', 'condition', 'code', 'agent', 'extractor', 'filter', 'subworkflow'];
  const selectedNode = builderNodes.find(n => n.id === selectedNodeId);

  const centerView = () => { setCanvasScroll({ x: 0, y: 0 }); };
  useEffect(() => { setTimeout(centerView, 100); }, []);

  return (
    <div className="builder">
      <div className="builder-toolbar">
        <button className="toolbar-btn" onClick={closeBuilder} title="Back">←</button>
        <span className="toolbar-divider" />
        <button className="toolbar-btn" onClick={undo} title="Undo">↶</button>
        <button className="toolbar-btn" onClick={redo} title="Redo">↷</button>
        <span className="toolbar-divider" />
        <span className="builder-title">{editingTemplateId ? 'Edit: ' + (templates.find(t => t.id === editingTemplateId)?.name || '') : 'New Workflow'}</span>
        <button className="btn btn-outline btn-sm" style={{ marginRight: 4 }}>▶ Test Run</button>
        <button className="btn btn-success btn-sm" onClick={openSave}>Save</button>
      </div>
      <div className="builder-body">
        <div className="builder-canvas" ref={canvasRef} onMouseDown={handleCanvasMouseDown}>
          <div className="canvas-viewport" style={{ width: 2000, height: 2000, transform: `translate(${-canvasScroll.x}px,${-canvasScroll.y}px)` }}>
            <svg className="canvas-svg" style={{ width: 2000, height: 2000 }}>
              <defs><marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0,0 L10,5 L0,10" fill="#94a3b8" /></marker></defs>
              {builderEdges.map((e, i) => {
                const fp = pos[e.from], tp = pos[e.to];
                if (!fp || !tp) return null;
                const nw = e.from === '_start' ? 140 : (e.from === '_start' ? 140 : 172);
                const x1 = e.from === '_start' ? fp.x + 140 : fp.x, y1 = fp.y + 24;
                const x2 = tp.x, y2 = tp.y + 24, cx = (x1 + x2) / 2;
                const d = `M${x1},${y1} C${cx},${y1} ${cx},${y2} ${x2},${y2}`;
                const isData = e.edge_type === 'data';
                return <path key={i} d={d} stroke={isData ? '#8b5cf6' : '#94a3b8'}
                  strokeWidth={2} fill="none" strokeDasharray={isData ? '6,4' : '8,4'}
                  markerEnd={isData ? undefined : 'url(#arrow)'} />;
              })}
              {/* Connection line while dragging */}
              {connectFrom && (() => {
                const fp = pos[connectFrom];
                if (!fp) return null;
                const nw = connectFrom === '_start' ? 140 : 172;
                const x1 = connectFrom === '_start' ? fp.x + 140 : fp.x, y1 = fp.y + 24;
                const x2 = connectPos.x, y2 = connectPos.y, cx = (x1 + x2) / 2;
                const d = `M${x1},${y1} C${cx},${y1} ${cx},${y2} ${x2},${y2}`;
                return <path d={d} stroke={connectType === 'data' ? '#8b5cf6' : '#0f3460'}
                  strokeWidth={2.5} fill="none" strokeDasharray="6,4" opacity={0.8} />;
              })()}
            </svg>

            {/* START node */}
            <div className={`node ${selectedNodeId === '_start' ? 'selected' : ''}`}
              style={{ left: pos._start.x, top: pos._start.y, minWidth: 140, borderColor: '#059669' }}
              onMouseDown={e => handleNodeMouseDown(e, '_start')}
              onClick={() => handleNodeClick('_start')}
              data-id="_start">
              <span className="node-type-icon" style={{ background: '#059669' }}>▶</span>
              <span className="node-name" style={{ fontWeight: 600 }}>START</span>
              <span style={{ fontSize: '0.65rem', color: '#94a3b8', cursor: 'pointer', marginLeft: 'auto' }}
                onClick={e => { e.stopPropagation(); setStartType(startType === 'User Input' ? 'Schedule' : 'User Input'); }}>{startType} ▾</span>
              <span className="node-port output" style={{ right: -11 }}
                onMouseDown={e => { e.stopPropagation(); e.preventDefault(); setConnectFrom('_start'); setConnectType('flow'); setConnectPort(''); }} />
            </div>

            {/* User nodes */}
            {builderNodes.map(n => {
              const p = pos[n.id];
              if (!p) return null;
              const color = NODE_COLORS[n.type] || '#64748b';
              const isCond = n.type === 'condition';
              return (
                <div key={n.id} className={`node ${selectedNodeId === n.id ? 'selected' : ''}`}
                  style={{ left: p.x, top: p.y, borderColor: color, minWidth: isCond ? 190 : 160, paddingRight: isCond ? 60 : 0 }}
                  onMouseDown={e => handleNodeMouseDown(e, n.id)}
                  onClick={() => handleNodeClick(n.id)}
                  data-id={n.id} data-type={n.type}>
                  <span className="node-type-icon" style={{ background: color }}>{n.type[0].toUpperCase()}</span>
                  <span className="node-name">{n.id}</span>
                  <span className="node-port input" />
                  {isCond ? (
                    <div className="node-ports-right">
                      <div className="node-port port-if" onMouseDown={e => { e.stopPropagation(); e.preventDefault(); setConnectFrom(n.id); setConnectType('flow'); setConnectPort('true'); }}>IF</div>
                      <div className="node-port port-else" onMouseDown={e => { e.stopPropagation(); e.preventDefault(); setConnectFrom(n.id); setConnectType('flow'); setConnectPort('false'); }}>ELSE</div>
                    </div>
                  ) : (
                    <span className="node-port output" onMouseDown={e => { e.stopPropagation(); e.preventDefault(); setConnectFrom(n.id); setConnectType('flow'); setConnectPort(''); }} />
                  )}
                  {(n.type === 'agent' || n.type === 'extractor') && (
                    <span className="node-port data" onMouseDown={e => { e.stopPropagation(); e.preventDefault(); setConnectFrom(n.id); setConnectType('data'); setConnectPort(''); }} />
                  )}
                </div>
              );
            })}

            {/* Ghost node */}
            {ghostNode && (
              <div className="node" style={{ left: ghostPos.x, top: ghostPos.y, opacity: 0.7, zIndex: 100, pointerEvents: 'none', borderColor: NODE_COLORS[ghostNode.type] || '#64748b' }}>
                <span className="node-type-icon" style={{ background: NODE_COLORS[ghostNode.type] || '#64748b' }}>{ghostNode.type[0].toUpperCase()}</span>
                <span className="node-name">{ghostNode.id}</span>
              </div>
            )}
          </div>
        </div>

        {/* Left sidebar */}
        <div className="sidebar">
          <div className="sidebar-btn" onClick={() => setAddPopup(!addPopup)} title="Add node">
            <div className="icon" style={{ background: '#0f3460', fontSize: '1.2rem' }}>+</div>
          </div>
          {addPopup && (
            <div className="add-popup active" style={{ top: 56, left: 58 }} onClick={e => e.stopPropagation()}>
              <div style={{ fontSize: '0.7rem', color: '#94a3b8', padding: '4px 14px 2px', textTransform: 'uppercase' }}>Regular</div>
              {['call', 'condition', 'code', 'filter', 'subworkflow'].map(t => (
                <div key={t} className="add-popup-item" onClick={() => {
                  const n = builderNodes.filter(x => x.type === t).length + 1;
                  const node: any = { id: `${t}_${n}`, type: t };
                  if (t === 'call') { node.webhook_url = 'https://httpbin.org/post'; node.method = 'POST'; node.body_type = 'json'; }
                  if (t === 'condition') node.expression = 'state._global.score >= 60';
                  if (t === 'code') { node.language = 'js'; node.code = '// your code here\nreturn data;'; }
                  if (t === 'filter') node.expression = 'data.students';
                  s.pushUndo();
                  setGhostNode(node);
                  setAddPopup(false);
                }}>
                  <span className="picon" style={{ background: NODE_COLORS[t] }}>{t[0].toUpperCase()}</span> {t.charAt(0).toUpperCase() + t.slice(1)}
                </div>
              ))}
              <div style={{ fontSize: '0.7rem', color: '#94a3b8', padding: '4px 14px 2px', textTransform: 'uppercase', borderTop: '1px solid #f1f5f9', marginTop: 2 }}>Agent</div>
              {['agent', 'extractor', 'subworkflow'].map(t => (
                <div key={t} className="add-popup-item" onClick={() => {
                  const n = builderNodes.filter(x => x.type === t).length + 1;
                  const node: any = { id: `${t}_${n}`, type: t };
                  if (t === 'extractor') { node.description = 'Extract data from file'; node.extract_prompt = 'Extract key information'; node.llm_profile = ''; }
                  if (t === 'agent') node.agent_config = { profile: '', system_prompt: 'You are a helpful assistant.', tools: [], max_turns: 10 };
                  if (t === 'subworkflow') node.sub_workflow_template_id = '';
                  s.pushUndo();
                  setGhostNode(node);
                  setAddPopup(false);
                }}>
                  <span className="picon" style={{ background: NODE_COLORS[t] }}>{t[0].toUpperCase()}</span> {t === 'subworkflow' ? 'Sub-Workflow' : t.charAt(0).toUpperCase() + t.slice(1)}
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Right properties panel */}
        <div className="right-panel" style={{ width: panelWidth, transform: panelCollapsed ? 'translateX(100%)' : '' }}>
          <div className="panel-resize-handle"
            onMouseDown={e => {
              e.preventDefault();
              const startX = e.clientX, startW = panelWidth;
              const onMove = (ev: MouseEvent) => { const w = startW - (ev.clientX - startX); if (w >= 200 && w <= 600) setPanelWidth(w); };
              const onUp = () => { document.removeEventListener('mousemove', onMove); document.removeEventListener('mouseup', onUp); };
              document.addEventListener('mousemove', onMove);
              document.addEventListener('mouseup', onUp);
            }} />
          <div className="right-panel-header">
            <span>Properties</span>
            <button className="panel-close-btn" onClick={() => setPanelCollapsed(!panelCollapsed)}>✕</button>
          </div>
          <div className="node-properties">
            {selectedNodeId === '_start' ? (
              <div className="props-card">
                <div className="props-title">START</div>
                <div className="form-group">
                  <label>Trigger Type</label>
                  <select value={startType} onChange={e => setStartType(e.target.value)}>
                    <option value="User Input">User Input</option>
                    <option value="Schedule">Schedule</option>
                  </select>
                </div>
                {startType === 'User Input' && (
                  <div className="form-group">
                    <label>Input Data</label>
                    <textarea rows={4} value={startInput} onChange={e => useStore.setState({ startInput: e.target.value })} />
                    <p className="form-hint">Enter plain text or JSON data to pass to workflow nodes.</p>
                  </div>
                )}
                {startType === 'Schedule' && (
                  <div className="form-group">
                    <label>Schedule Mode</label>
                    <select defaultValue="daily" onChange={e => useStore.setState({ cronExpr: e.target.value === 'daily' ? '0 9 * * *' : '*/60 * * * *' })}>
                      <option value="daily">Daily at 09:00</option>
                      <option value="every">Every 60 minutes</option>
                    </select>
                  </div>
                )}
              </div>
            ) : selectedNode ? (
              <div className="props-card">
                <div className="props-header">
                  <span className="props-type" style={{ background: NODE_COLORS[selectedNode.type] || '#64748b' }}>{selectedNode.type.toUpperCase()}</span>
                  <button className="btn btn-xs btn-danger" onClick={() => { deleteNode(selectedNode.id); useStore.setState({ selectedNodeId: null }); }}>Delete</button>
                </div>
                <div className="form-group">
                  <label>ID</label>
                  <input value={selectedNode.id} onChange={e => updateNode('id', e.target.value)} />
                </div>
                {selectedNode.type === 'call' && (
                  <>
                    <div className="form-group"><label>Description</label><input value={selectedNode.description || ''} onChange={e => updateNode('description', e.target.value)} /></div>
                    <div className="form-group"><label>API</label>
                      <div style={{ display: 'flex', gap: 4 }}>
                        <div style={{ flex: '0 0 90px' }}><select value={selectedNode.method || 'GET'} onChange={e => updateNode('method', e.target.value)}><option>GET</option><option>POST</option><option>PUT</option><option>DELETE</option><option>PATCH</option></select></div>
                        <div style={{ flex: 1 }}><input value={selectedNode.webhook_url || ''} placeholder="https://api.example.com" onChange={e => updateNode('webhook_url', e.target.value)} /></div>
                      </div>
                    </div>
                    <div className="form-group"><label>Body Type</label><select value={selectedNode.body_type || 'none'} onChange={e => updateNode('body_type', e.target.value)}><option value="none">None</option><option value="raw">Raw</option><option value="json">JSON</option></select></div>
                    {selectedNode.body_type && selectedNode.body_type !== 'none' && <div className="form-group"><label>Body Content</label><textarea rows={4} value={selectedNode.body_content || ''} onChange={e => updateNode('body_content', e.target.value)} /></div>}
                  </>
                )}
                {selectedNode.type === 'code' && (
                  <>
                    <div className="form-group"><label>Language</label><select value={selectedNode.language || 'js'} onChange={e => updateNode('language', e.target.value)}><option value="js">JavaScript</option><option value="python">Python</option></select></div>
                    <div className="form-group"><label>Code</label><textarea rows={5} value={selectedNode.code || ''} onChange={e => updateNode('code', e.target.value)} /></div>
                  </>
                )}
                {selectedNode.type === 'condition' && <div className="form-group"><label>Expression</label><input value={selectedNode.expression || ''} onChange={e => updateNode('expression', e.target.value)} /></div>}
                {selectedNode.type === 'filter' && <div className="form-group"><label>Expression</label><input value={selectedNode.expression || ''} onChange={e => updateNode('expression', e.target.value)} /><p className="form-hint">Filters data based on expression.</p></div>}
                {selectedNode.type === 'agent' && <AgentProps />}
                {selectedNode.type === 'subworkflow' && <SubWorkflowProps />}
                {selectedNode.type === 'extractor' && <ExtractorProps />}
              </div>
            ) : (
              <div className="empty-state"><p>Select a node to edit.</p></div>
            )}
          </div>
        </div>
      </div>

      {/* Save modal */}
      {saveOpen && (
        <div className="modal-overlay active" onClick={() => setSaveOpen(false)}>
          <div className="modal" style={{ maxWidth: 420 }} onClick={e => e.stopPropagation()}>
            <h2>{editingTemplateId ? 'Update Workflow' : 'Save Workflow'}</h2>
            <div className="form-group"><label>Workflow Name</label><input value={saveName} placeholder="my_workflow" onChange={e => setSaveName(e.target.value)} /></div>
            <div className="form-group"><label>Description</label><input value={saveDesc} placeholder="What does this workflow do?" onChange={e => setSaveDesc(e.target.value)} /></div>
            <div className="modal-actions">
              <button className="btn btn-outline" onClick={() => setSaveOpen(false)}>Cancel</button>
              <button className="btn btn-success" onClick={handleSave}>{editingTemplateId ? 'Update' : 'Save'}</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function AgentProps() {
  const { updateAgent, builderNodes, selectedNodeId } = useStore();
  const node = builderNodes.find(n => n.id === selectedNodeId);
  const a = node?.agent_config || { profile: '', system_prompt: '', tools: [], max_turns: 10 };
  const [profiles, setProfiles] = useState<string[]>([]);
  useEffect(() => { fetch('/api/v1/llm/profiles').then(r => r.json()).then(setProfiles).catch(() => {}); }, []);
  return (
    <>
      <div className="form-group"><label>LLM Profile</label><select value={a.profile} onChange={e => updateAgent('profile', e.target.value)}><option value="">Select...</option>{profiles.map(p => <option key={p} value={p}>{p}</option>)}</select></div>
      <div className="form-group"><label>System Prompt</label><textarea rows={3} value={a.system_prompt} onChange={e => updateAgent('system_prompt', e.target.value)} /></div>
      <div className="form-group"><label>Max Turns</label><input type="number" value={a.max_turns} onChange={e => updateAgent('max_turns', parseInt(e.target.value) || 10)} /></div>
      <div className="perm-section">
        <label className="perm-label">Permissions</label>
        <label className="perm-check"><input type="checkbox" checked={!!a.enable_read_tools} onChange={e => updateAgent('enable_read_tools', e.target.checked)} /> Read local files</label>
        <label className="perm-check"><input type="checkbox" checked={!!a.enable_write_tools} onChange={e => updateAgent('enable_write_tools', e.target.checked)} /> Write local files</label>
        <label className="perm-check"><input type="checkbox" checked={!!a.enable_web_tools} onChange={e => updateAgent('enable_web_tools', e.target.checked)} /> Web access (search/fetch)</label>
      </div>
    </>
  );
}

function SubWorkflowProps() {
  const { updateNode, builderNodes, selectedNodeId, templates } = useStore();
  const node = builderNodes.find(n => n.id === selectedNodeId);
  return (
    <>
      <div className="form-group"><label>Template</label><select value={node?.sub_workflow_template_id || ''} onChange={e => updateNode('sub_workflow_template_id', e.target.value)}><option value="">Select...</option>{templates.map(t => <option key={t.id} value={t.id}>{t.name}</option>)}</select></div>
      <div className="form-group"><label>Input Key</label><input value={node?.sub_workflow_input_key || ''} placeholder="data.students" onChange={e => updateNode('sub_workflow_input_key', e.target.value)} /><p className="form-hint">State key to pass as input</p></div>
      <div className="form-group"><label>Output Key</label><input value={node?.sub_workflow_output_key || ''} placeholder="data.result" onChange={e => updateNode('sub_workflow_output_key', e.target.value)} /><p className="form-hint">State key to store result</p></div>
    </>
  );
}

function ExtractorProps() {
  const { updateNode, builderNodes, selectedNodeId } = useStore();
  const node = builderNodes.find(n => n.id === selectedNodeId);
  const [profiles, setProfiles] = useState<string[]>([]);
  useEffect(() => { fetch('/api/v1/llm/profiles').then(r => r.json()).then(setProfiles).catch(() => {}); }, []);
  return (
    <>
      <div className="form-group"><label>Description</label><input value={node?.description || ''} onChange={e => updateNode('description', e.target.value)} /></div>
      <div className="form-group"><label>LLM Profile</label><select value={node?.llm_profile || ''} onChange={e => updateNode('llm_profile', e.target.value)}><option value="">Select...</option>{profiles.map(p => <option key={p} value={p}>{p}</option>)}</select></div>
      <div className="form-group"><label>Extract Prompt</label><textarea rows={3} value={node?.extract_prompt || ''} onChange={e => updateNode('extract_prompt', e.target.value)} /></div>
      <div className="form-group"><label>File</label><input type="file" /></div>
    </>
  );
}