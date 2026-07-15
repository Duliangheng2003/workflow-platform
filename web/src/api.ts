import type { Template, Instance, LLMProfile } from './types';

const API = '';

async function api<T>(path: string, options?: RequestInit): Promise<T> {
  const r = await fetch(API + path, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  });
  if (r.status === 204) return {} as T;
  return r.json();
}

export const templatesApi = {
  list: () => api<Template[]>('/api/v1/templates'),
  get: (id: string) => api<Template>(`/api/v1/templates/${id}`),
  create: (body: any) => api<Template>('/api/v1/templates', { method: 'POST', body: JSON.stringify(body) }),
  update: (id: string, body: any) => api<Template>(`/api/v1/templates/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  delete: (id: string) => api<void>(`/api/v1/templates/${id}`, { method: 'DELETE' }),
};

export const instancesApi = {
  list: () => api<Instance[]>('/api/v1/instances'),
  get: (id: string) => api<Instance>(`/api/v1/instances/${id}`),
  start: (templateId: string, input: any) => api<Instance>(`/api/v1/templates/${templateId}/instances`, { method: 'POST', body: JSON.stringify({ input }) }),
  delete: (id: string) => api<void>(`/api/v1/instances/${id}`, { method: 'DELETE' }),
};

export const llmApi = {
  profiles: () => api<string[]>('/api/v1/llm/profiles'),
};