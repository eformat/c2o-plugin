import {
  AgentInfo,
  NamespaceInfo,
  CredentialInfo,
  DeployRequest,
  DeployResponse,
  ConnectionInfo,
  CreateCredentialsRequest,
  PodInfo,
} from './types';

const BASE_PATH = '/api/proxy/plugin/c2o-plugin/backend/api';

function getCsrfToken(): string {
  const match = document.cookie.match(/csrf-token=([^;]+)/);
  return match ? match[1] : '';
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...((options.headers as Record<string, string>) || {}),
  };

  const csrfToken = getCsrfToken();
  if (csrfToken) {
    headers['X-CSRFToken'] = csrfToken;
  }

  const response = await fetch(`${BASE_PATH}${path}`, {
    ...options,
    headers,
  });

  if (!response.ok) {
    const body = await response.json().catch(() => ({ error: response.statusText }));
    throw new Error(body.error || `Request failed: ${response.status}`);
  }

  return response.json();
}

export async function listNamespaces(): Promise<NamespaceInfo[]> {
  return request<NamespaceInfo[]>('/namespaces');
}

export async function createNamespace(name: string): Promise<void> {
  await request('/namespaces', {
    method: 'POST',
    body: JSON.stringify({ name }),
  });
}

export async function listAgents(namespace: string): Promise<AgentInfo[]> {
  return request<AgentInfo[]>(`/agents?namespace=${encodeURIComponent(namespace)}`);
}

export async function deleteAgent(name: string, namespace: string): Promise<void> {
  await request(`/agents/${encodeURIComponent(name)}?namespace=${encodeURIComponent(namespace)}`, {
    method: 'DELETE',
  });
}

export async function deploy(req: DeployRequest): Promise<DeployResponse> {
  return request<DeployResponse>('/deploy', {
    method: 'POST',
    body: JSON.stringify(req),
  });
}

export async function createCredentials(req: CreateCredentialsRequest): Promise<{ status: string; name: string }> {
  return request('/credentials', {
    method: 'POST',
    body: JSON.stringify(req),
  });
}

export async function listCredentials(namespace: string): Promise<CredentialInfo[]> {
  return request<CredentialInfo[]>(`/credentials?namespace=${encodeURIComponent(namespace)}`);
}

export async function getConnection(namespace: string): Promise<ConnectionInfo> {
  return request<ConnectionInfo>(`/connection?namespace=${encodeURIComponent(namespace)}`);
}

export async function scaleAgent(name: string, namespace: string, replicas: number): Promise<void> {
  await request(`/agents/${encodeURIComponent(name)}/scale?namespace=${encodeURIComponent(namespace)}`, {
    method: 'PATCH',
    body: JSON.stringify({ replicas }),
  });
}

export async function getAgentPod(name: string, namespace: string): Promise<PodInfo> {
  return request<PodInfo>(`/agents/${encodeURIComponent(name)}/pod?namespace=${encodeURIComponent(namespace)}`);
}

export async function addAgent(req: {
  namespace: string;
  agentType?: string;
  prefix?: string;
  credentialName?: string;
  image?: string;
}): Promise<{ status: string; name: string; instance: string }> {
  return request('/agents/add', {
    method: 'POST',
    body: JSON.stringify(req),
  });
}
