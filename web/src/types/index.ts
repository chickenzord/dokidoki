export type NodeStatus = 'alive' | 'suspect' | 'offline';

export interface Node {
  id: string;
  name: string;
  addresses: string[];
  status: NodeStatus;
  version: string;
  is_self: boolean;
  last_seen?: string;
}

export interface ContainerRollup {
  running: number;
  exited: number;
  restarting: number;
  paused: number;
  dead: number;
  total: number;
}

export interface StackSummary {
  name: string;
  source: 'managed' | 'external';
  composePresent: boolean;
  composePath?: string;
  rollup: ContainerRollup;
  services: string[];
  is_self?: boolean;
}

export interface PortMapping {
  ip?: string;
  privatePort: number;
  publicPort?: number;
  type: string;
}

export interface ContainerSummary {
  id: string;
  name: string;
  image: string;
  state: string;
  status: string;
  created: number;
  ports: PortMapping[];
  labels: Record<string, string>;
  stack?: string;
  service?: string;
  is_self?: boolean;
}

export interface DockerHostInfo {
  engineVersion: string;
  apiVersion: string;
  os: string;
  arch: string;
  containers: number;
  containersRunning: number;
  containersPaused: number;
  containersStopped: number;
}

export interface HostInfo {
  hostname: string;
  os: string;
  arch: string;
  dokidokiVersion: string;
  stacksDir: string;
  docker: DockerHostInfo;
}

export interface PingResponse {
  status: string;
  error?: string;
}
