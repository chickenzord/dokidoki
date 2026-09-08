import {
  HostInfo,
  Node,
  PingResponse,
  StackSummary,
  ContainerSummary,
  StackFile,
  StackFilesResponse,
  CreateStackRequest,
  ContainerComposeResponse,
  EnrichedContainerInspect,
  OperationResult,
} from '../types';

class ApiService {
  private activeEndpoint: string = '';
  private endpointChangeListeners: Array<(endpoint: string) => void> = [];

  constructor() {
    // Check localStorage for saved endpoint
    const saved = localStorage.getItem('dokidoki_active_endpoint');
    if (saved) {
      this.activeEndpoint = saved;
    }
  }

  public getActiveEndpoint(): string {
    return this.activeEndpoint;
  }

  public setActiveEndpoint(endpoint: string) {
    // Normalize endpoint (trim trailing slash)
    const normalized = endpoint.replace(/\/+$/, '');
    this.activeEndpoint = normalized;
    if (normalized) {
      localStorage.setItem('dokidoki_active_endpoint', normalized);
    } else {
      localStorage.removeItem('dokidoki_active_endpoint');
    }
    this.notifyEndpointChange(normalized);
  }

  public onEndpointChange(listener: (endpoint: string) => void): () => void {
    this.endpointChangeListeners.push(listener);
    return () => {
      this.endpointChangeListeners = this.endpointChangeListeners.filter(l => l !== listener);
    };
  }

  private notifyEndpointChange(endpoint: string) {
    for (const listener of this.endpointChangeListeners) {
      try {
        listener(endpoint);
      } catch (e) {
        console.error('Error in endpoint change listener:', e);
      }
    }
  }

  private resolveUrl(path: string, customEndpoint?: string): string {
    const base = customEndpoint !== undefined ? customEndpoint : this.activeEndpoint;
    const cleanPath = path.startsWith('/') ? path : `/${path}`;
    return base ? `${base}${cleanPath}` : cleanPath;
  }

  /**
   * Performs a fetch with transparent error handling and fallback to local endpoint
   * if a remote node is unreachable.
   */
  public async request<T>(path: string, options?: RequestInit, customEndpoint?: string): Promise<T> {
    const url = this.resolveUrl(path, customEndpoint);

    try {
      const response = await fetch(url, {
        ...options,
        headers: {
          'Accept': 'application/json',
          ...(options?.headers || {}),
        },
      });

      if (!response.ok) {
        const errorBody = await response.json().catch(() => null);
        const errorMsg = errorBody?.error || `HTTP ${response.status}: ${response.statusText}`;
        throw new Error(errorMsg);
      }

      return await response.json();
    } catch (err: any) {
      // If we failed against a remote custom endpoint, check if we should transparently fall back
      const isRemote = (customEndpoint !== undefined ? customEndpoint : this.activeEndpoint) !== '';
      if (isRemote && !customEndpoint) {
        console.warn(`Request to active remote node (${this.activeEndpoint}) failed: ${err.message}. Falling back to local node.`);
        const localUrl = this.resolveUrl(path, '');
        const localResp = await fetch(localUrl, {
          ...options,
          headers: {
            'Accept': 'application/json',
            ...(options?.headers || {}),
          },
        });
        if (!localResp.ok) {
          throw new Error(`Fallback failed: HTTP ${localResp.status}`);
        }
        return await localResp.json();
      }
      throw err;
    }
  }

  public async ping(customEndpoint?: string): Promise<PingResponse> {
    try {
      return await this.request<PingResponse>('/api/v1/host/ping', { method: 'GET' }, customEndpoint);
    } catch (err: any) {
      return { status: 'error', error: err.message };
    }
  }

  public async getHostInfo(customEndpoint?: string): Promise<HostInfo> {
    return this.request<HostInfo>('/api/v1/host', { method: 'GET' }, customEndpoint);
  }

  public async getNodes(customEndpoint?: string): Promise<Node[]> {
    return this.request<Node[]>('/api/v1/nodes', { method: 'GET' }, customEndpoint);
  }

  public async getStacks(customEndpoint?: string, source?: string): Promise<StackSummary[]> {
    const query = source && source !== 'all' ? `?source=${encodeURIComponent(source)}` : '';
    return this.request<StackSummary[]>(`/api/v1/stacks${query}`, { method: 'GET' }, customEndpoint);
  }

  public async getContainers(customEndpoint?: string, stack?: string): Promise<ContainerSummary[]> {
    const query = stack ? `?stack=${encodeURIComponent(stack)}` : '';
    return this.request<ContainerSummary[]>(`/api/v1/containers${query}`, { method: 'GET' }, customEndpoint);
  }

  public async getContainer(id: string, customEndpoint?: string): Promise<ContainerSummary> {
    return this.request<ContainerSummary>(`/api/v1/containers/${encodeURIComponent(id)}`, { method: 'GET' }, customEndpoint);
  }

  public async inspectContainer(id: string, customEndpoint?: string): Promise<EnrichedContainerInspect> {
    return this.request<EnrichedContainerInspect>(`/api/v1/containers/${encodeURIComponent(id)}`, { method: 'GET' }, customEndpoint);
  }

  public async createStack(data: CreateStackRequest, customEndpoint?: string): Promise<StackSummary> {
    return this.request<StackSummary>('/api/v1/stacks', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }, customEndpoint);
  }

  public async updateStack(name: string, data: Partial<CreateStackRequest>, customEndpoint?: string): Promise<StackSummary> {
    return this.request<StackSummary>(`/api/v1/stacks/${encodeURIComponent(name)}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }, customEndpoint);
  }

  public async getStackFiles(name: string, customEndpoint?: string): Promise<StackFilesResponse> {
    return this.request<StackFilesResponse>(`/api/v1/stacks/${encodeURIComponent(name)}/files`, { method: 'GET' }, customEndpoint);
  }

  public async getStackFile(name: string, filename: string, customEndpoint?: string): Promise<StackFile> {
    return this.request<StackFile>(`/api/v1/stacks/${encodeURIComponent(name)}/files/${encodeURIComponent(filename)}`, { method: 'GET' }, customEndpoint);
  }

  public async getContainerCompose(id: string, customEndpoint?: string): Promise<ContainerComposeResponse> {
    return this.request<ContainerComposeResponse>(`/api/v1/containers/${encodeURIComponent(id)}/compose`, { method: 'GET' }, customEndpoint);
  }

  /**
   * Performs an operation that can stream output or return JSON.
   */
  public async streamOperation(
    path: string,
    onChunk?: (text: string) => void,
    customEndpoint?: string
  ): Promise<OperationResult> {
    const streamQuery = onChunk ? (path.includes('?') ? '&stream=true' : '?stream=true') : '';
    const url = this.resolveUrl(`${path}${streamQuery}`, customEndpoint);

    const response = await fetch(url, {
      method: 'POST',
      headers: {
        Accept: onChunk ? 'text/plain' : 'application/json',
      },
    });

    if (!response.ok) {
      const errBody = await response.json().catch(() => null);
      throw new Error(errBody?.error || `HTTP ${response.status}: ${response.statusText}`);
    }

    if (onChunk && response.body) {
      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let fullOutput = '';
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        const chunk = decoder.decode(value, { stream: true });
        fullOutput += chunk;
        onChunk(chunk);
      }
      return {
        success: true,
        message: 'Operation completed successfully',
        output: fullOutput,
      };
    }

    return await response.json();
  }

  public async restartContainer(id: string, force = false, customEndpoint?: string): Promise<OperationResult> {
    const q = force ? '?force=true' : '';
    return this.request<OperationResult>(`/api/v1/containers/${encodeURIComponent(id)}/restart${q}`, { method: 'POST' }, customEndpoint);
  }

  public async startContainer(id: string, customEndpoint?: string): Promise<OperationResult> {
    return this.request<OperationResult>(`/api/v1/containers/${encodeURIComponent(id)}/start`, { method: 'POST' }, customEndpoint);
  }

  public async stopContainer(id: string, force = false, customEndpoint?: string): Promise<OperationResult> {
    const q = force ? '?force=true' : '';
    return this.request<OperationResult>(`/api/v1/containers/${encodeURIComponent(id)}/stop${q}`, { method: 'POST' }, customEndpoint);
  }

  public async pullContainer(id: string, onChunk?: (text: string) => void, customEndpoint?: string): Promise<OperationResult> {
    return this.streamOperation(`/api/v1/containers/${encodeURIComponent(id)}/pull`, onChunk, customEndpoint);
  }

  public async composeUp(name: string, onChunk?: (text: string) => void, customEndpoint?: string): Promise<OperationResult> {
    return this.streamOperation(`/api/v1/stacks/${encodeURIComponent(name)}/up`, onChunk, customEndpoint);
  }

  public async composeDown(name: string, onChunk?: (text: string) => void, customEndpoint?: string): Promise<OperationResult> {
    return this.streamOperation(`/api/v1/stacks/${encodeURIComponent(name)}/down`, onChunk, customEndpoint);
  }

  public async composeRestart(name: string, service?: string, onChunk?: (text: string) => void, customEndpoint?: string): Promise<OperationResult> {
    const q = service ? `?service=${encodeURIComponent(service)}` : '';
    return this.streamOperation(`/api/v1/stacks/${encodeURIComponent(name)}/restart${q}`, onChunk, customEndpoint);
  }

  public async composePull(name: string, onChunk?: (text: string) => void, customEndpoint?: string): Promise<OperationResult> {
    return this.streamOperation(`/api/v1/stacks/${encodeURIComponent(name)}/pull`, onChunk, customEndpoint);
  }
}

export const api = new ApiService();

