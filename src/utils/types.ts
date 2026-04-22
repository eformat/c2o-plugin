export interface AgentInfo {
  name: string;
  namespace: string;
  instance: string;
  status: string;
  ready: boolean;
  image: string;
  age: string;
  agentType: string;
  deployedBy: string;
  replicas: number;
}

export interface NamespaceInfo {
  name: string;
  status: string;
}

export interface CredentialInfo {
  name: string;
  namespace: string;
  type: string;
  createdAt: string;
}

export interface DeployRequest {
  agentType: string;
  namespace: string;
  count: number;
  prefix: string;
  credentialName: string;
  image: string;
}

export interface DeployResponse {
  status: string;
  namespace: string;
  agents: string[];
}

export interface ConnectionInfo {
  mcpConfig: string;
  skillPrompt: string;
  installCmd: string;
  grafanaUrls: Record<string, string>;
}

export interface CreateCredentialsRequest {
  namespace: string;
  name: string;
  type: 'apikey' | 'gcpjson' | 'custom';
  data: Record<string, string>;
}

export type AgentType = 'claude' | 'codex' | 'opencode';

export interface AgentTypeOption {
  id: AgentType;
  name: string;
  description: string;
  enabled: boolean;
  comingSoon: boolean;
}

export const AGENT_TYPES: AgentTypeOption[] = [
  {
    id: 'claude',
    name: 'Claude Code',
    description: 'Anthropic Claude coding agent with full tool access',
    enabled: true,
    comingSoon: false,
  },
  {
    id: 'codex',
    name: 'Codex',
    description: 'OpenAI Codex coding agent',
    enabled: false,
    comingSoon: true,
  },
  {
    id: 'opencode',
    name: 'OpenCode',
    description: 'Open-source coding agent',
    enabled: false,
    comingSoon: true,
  },
];
