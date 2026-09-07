import { HostInfo, Node, PingResponse, StackSummary, ContainerSummary } from '../types';

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
}

export const api = new ApiService();
