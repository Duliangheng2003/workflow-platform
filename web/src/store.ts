import { create } from 'zustand';
import type { Node, Edge, Template, Instance, NodeState } from './types';
import { templatesApi, instancesApi } from './api';

export const NODE_COLORS: Record<string, string> = {
  call: '#3b82f6', agent: '#8b5cf6', condition: '#f59e0b',
  code: '#10b981', extractor: '#06b6d4', filter: '#0891b2', subworkflow: '#6366f1',
};

interface AppState {
  page: string;
  templates: Template[];
  instances: Instance[];
  toast: { msg: string; type: string } | null;

  // Builder state
  builderNodes: Node[];
  builderEdges: Edge[];
  selectedNodeId: string | null;
  nodePositions: Record<string, { x: number; y: number }>;
  startType: string;
  cronExpr: string;
  startInput: string;
  editingTemplateId: string | null;
  undoStack: any[];
  redoStack: any[];

  setPage: (p: string) => void;
  showToast: (msg: string, type?: string) => void;
  loadTemplates: () => Promise<void>;
  loadInstances: () => Promise<void>;
  startInstance: (id: string) => Promise<void>;
  deleteInstance: (id: string) => Promise<void>;
  deleteTemplate: (id: string) => Promise<void>;

  // Builder
  openBuilder: (data?: Template) => void;
  closeBuilder: () => void;
  setSelectedNode: (id: string | null) => void;
  addNode: (type: string) => void;
  deleteNode: (id: string) => void;
  addEdge: (from: string, to: string, edgeType: string, outputPort?: string) => void;
  updateNode: (key: string, value: any) => void;
  updateAgent: (key: string, value: any) => void;
  pushUndo: () => void;
  undo: () => void;
  redo: () => void;
  setStartType: (v: string) => void;
  saveWorkflow: (name: string, desc: string) => Promise<void>;
}

export const useStore = create<AppState>((set, get) => ({
  page: 'templates',
  templates: [],
  instances: [],
  toast: null,
  builderNodes: [],
  builderEdges: [],
  selectedNodeId: null,
  nodePositions: {},
  startType: 'User Input',
  cronExpr: '',
  startInput: '{}',
  editingTemplateId: null,
  undoStack: [],
  redoStack: [],

  setPage: (p) => {
    set({ page: p });
    if (p === 'templates') get().loadTemplates();
    if (p === 'instances') get().loadInstances();
  },
  showToast: (msg, type) => {
    set({ toast: { msg, type: type || 'info' } });
    setTimeout(() => set({ toast: null }), 3000);
  },
  loadTemplates: async () => {
    const templates = await templatesApi.list();
    set({ templates });
  },
  loadInstances: async () => {
    const instances = await instancesApi.list();
    set({ instances });
  },
  startInstance: async (id) => {
    const t = get().templates.find(x => x.id === id);
    let input = {};
    try { input = JSON.parse(t?.start_input || '{}'); } catch (e) { input = {}; }
    await instancesApi.start(id, input);
    get().showToast('Instance started', 'success');
    get().setPage('instances');
  },
  deleteInstance: async (id) => {
    await instancesApi.delete(id);
    get().loadInstances();
    get().showToast('Instance deleted', 'success');
  },
  deleteTemplate: async (id) => {
    await templatesApi.delete(id);
    get().loadTemplates();
    get().showToast('Template deleted', 'success');
  },

  openBuilder: (data) => {
    if (data) {
      set({
        builderNodes: JSON.parse(JSON.stringify(data.nodes || [])),
        builderEdges: JSON.parse(JSON.stringify(data.edges || [])),
        startType: data.start_type || 'User Input',
        cronExpr: data.cron_expr || '',
        startInput: data.start_input || '{}',
        editingTemplateId: data.id,
        selectedNodeId: null,
        nodePositions: {},
      });
    } else {
      set({
        builderNodes: [], builderEdges: [], selectedNodeId: null,
        nodePositions: {}, startType: 'User Input', cronExpr: '',
        startInput: '{}', editingTemplateId: null,
      });
    }
    set({ page: 'builder' });
  },
  closeBuilder: () => {
    set({ editingTemplateId: null, page: 'templates' });
  },
  setSelectedNode: (id) => set({ selectedNodeId: id }),
  addNode: (type) => {
    const state = get();
    const n = state.builderNodes.filter(x => x.type === type).length + 1;
    const node: Node = { id: `${type}_${n}`, type: type as any };
    if (type === 'call') { node.webhook_url = 'https://httpbin.org/post'; node.method = 'POST'; node.body_type = 'json'; }
    if (type === 'condition') node.expression = 'state._global.score >= 60';
    if (type === 'code') { node.language = 'js'; node.code = '// your code here\nreturn data;'; }
    if (type === 'extractor') { node.description = 'Extract data from file'; node.extract_prompt = 'Extract key information'; node.llm_profile = ''; }
    if (type === 'filter') node.expression = 'data.students';
    if (type === 'agent') node.agent_config = { profile: '', system_prompt: 'You are a helpful assistant.', tools: [], max_turns: 10 };
    if (type === 'subworkflow') node.sub_workflow_template_id = '';
    get().pushUndo();
    set({ builderNodes: [...state.builderNodes, node], selectedNodeId: node.id });
  },
  deleteNode: (id) => {
    get().pushUndo();
    const state = get();
    set({
      builderNodes: state.builderNodes.filter(n => n.id !== id),
      builderEdges: state.builderEdges.filter(e => e.from !== id && e.to !== id),
      selectedNodeId: state.selectedNodeId === id ? null : state.selectedNodeId,
    });
  },
  addEdge: (from, to, edgeType, outputPort) => {
    get().pushUndo();
    const state = get();
    if (state.builderEdges.find(e => e.from === from && e.to === to)) return;
    const out = state.builderEdges.filter(e => e.from === from && e.edge_type !== 'data').length;
    const ins = state.builderEdges.filter(e => e.to === to && e.edge_type !== 'data').length;
    if (out >= 4 || ins >= 2) { get().showToast('Connection limit', 'warning'); return; }
    set({ builderEdges: [...state.builderEdges, { from, to, edge_type: edgeType as any, output_port: outputPort || '' }] });
  },
  updateNode: (key, value) => {
    const state = get();
    const nodes = [...state.builderNodes];
    const n = nodes.find(x => x.id === state.selectedNodeId);
    if (!n) return;
    if (key === 'id' && value !== n.id) {
      state.builderEdges.forEach(e => { if (e.from === n.id) e.from = value; if (e.to === n.id) e.to = value; });
    }
    (n as any)[key] = value;
    set({ builderNodes: nodes, selectedNodeId: key === 'id' ? value : state.selectedNodeId });
  },
  updateAgent: (key, value) => {
    const state = get();
    const nodes = [...state.builderNodes];
    const n = nodes.find(x => x.id === state.selectedNodeId);
    if (!n) return;
    if (!n.agent_config) n.agent_config = { profile: '', system_prompt: '', tools: [], max_turns: 10 };
    (n.agent_config as any)[key] = value;
    set({ builderNodes: nodes });
  },
  pushUndo: () => {
    const state = get();
    const stack = [...state.undoStack, { nodes: JSON.parse(JSON.stringify(state.builderNodes)), edges: JSON.parse(JSON.stringify(state.builderEdges)), positions: JSON.parse(JSON.stringify(state.nodePositions)) }];
    if (stack.length > 50) stack.shift();
    set({ undoStack: stack, redoStack: [] });
  },
  undo: () => {
    const state = get();
    if (state.undoStack.length === 0) return;
    const next = state.undoStack[state.undoStack.length - 1];
    set({
      undoStack: state.undoStack.slice(0, -1),
      redoStack: [...state.redoStack, { nodes: JSON.parse(JSON.stringify(state.builderNodes)), edges: JSON.parse(JSON.stringify(state.builderEdges)), positions: JSON.parse(JSON.stringify(state.nodePositions)) }],
      builderNodes: next.nodes,
      builderEdges: next.edges,
      nodePositions: next.positions,
      selectedNodeId: null,
    });
  },
  redo: () => {
    const state = get();
    if (state.redoStack.length === 0) return;
    const next = state.redoStack[state.redoStack.length - 1];
    set({
      redoStack: state.redoStack.slice(0, -1),
      undoStack: [...state.undoStack, { nodes: JSON.parse(JSON.stringify(state.builderNodes)), edges: JSON.parse(JSON.stringify(state.builderEdges)), positions: JSON.parse(JSON.stringify(state.nodePositions)) }],
      builderNodes: next.nodes,
      builderEdges: next.edges,
      nodePositions: next.positions,
      selectedNodeId: null,
    });
  },
  setStartType: (v) => set({ startType: v }),
  saveWorkflow: async (name, desc) => {
    const state = get();
    const body = {
      name, description: desc,
      nodes: state.builderNodes,
      edges: state.builderEdges.map(e => ({ from: e.from, to: e.to, edge_type: e.edge_type, output_port: e.output_port || undefined })),
      start_type: state.startType,
      cron_expr: state.startType === 'Schedule' ? state.cronExpr : undefined,
      start_input: state.startInput,
    };
    if (state.editingTemplateId) {
      await templatesApi.update(state.editingTemplateId, body);
      get().showToast('Workflow updated!', 'success');
    } else {
      await templatesApi.create(body);
      get().showToast('Workflow saved!', 'success');
    }
    set({ editingTemplateId: null, page: 'templates' });
    get().loadTemplates();
  },
}));