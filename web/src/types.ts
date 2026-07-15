export interface Node {
  id: string;
  type: 'call' | 'condition' | 'code' | 'agent' | 'extractor' | 'filter' | 'subworkflow';
  description?: string;
  webhook_url?: string;
  method?: string;
  body_type?: string;
  body_content?: string;
  expression?: string;
  language?: string;
  code?: string;
  file_content?: string;
  file_name?: string;
  extract_prompt?: string;
  llm_profile?: string;
  agent_config?: AgentConfig;
  sub_workflow_template_id?: string;
  sub_workflow_input_key?: string;
  sub_workflow_output_key?: string;
}

export interface AgentConfig {
  profile: string;
  system_prompt: string;
  tools: string[];
  max_turns: number;
  enable_read_tools?: boolean;
  enable_write_tools?: boolean;
  enable_web_tools?: boolean;
}

export interface Edge {
  from: string;
  to: string;
  edge_type: 'flow' | 'data';
  output_port?: string;
}

export interface Template {
  id: string;
  name: string;
  description: string;
  nodes: Node[];
  edges: Edge[];
  start_type: string;
  cron_expr?: string;
  start_input?: string;
  last_run_at?: string;
  created_at: string;
  updated_at: string;
}

export interface Instance {
  id: string;
  template_id: string;
  status: 'running' | 'completed' | 'failed' | 'pending' | 'paused';
  state: Record<string, any>;
  node_states: Record<string, NodeState>;
  current_node_id?: string;
  error?: string;
  created_at: string;
  updated_at: string;
}

export interface NodeState {
  node_id: string;
  status: string;
  output?: any;
  error?: string;
}

export interface LLMProfile {
  name: string;
}