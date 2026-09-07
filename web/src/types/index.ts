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

export interface ClusterStack extends StackSummary {
  hostId: string;
  hostName: string;
  hostEndpoint: string;
}

export interface ClusterContainer extends ContainerSummary {
  hostId: string;
  hostName: string;
  hostEndpoint: string;
}

export interface StackFile {
  name: string;
  path: string;
  size: number;
  content?: string;
  isCompose: boolean;
  isEnv: boolean;
}

export interface StackFilesResponse {
  stack: string;
  dir: string;
  files: StackFile[];
}

export interface CreateStackRequest {
  name: string;
  content?: string;
  composePath?: string;
}

export interface ContainerComposeResponse {
  stackName: string;
  content: string;
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

export interface ContainerMount {
  Type?: string;
  Name?: string;
  Source: string;
  Destination: string;
  Mode?: string;
  RW?: boolean;
  Propagation?: string;
}

export interface ContainerPortBinding {
  HostIp: string;
  HostPort: string;
}

export interface EnrichedContainerInspect {
  Id: string;
  Created: string;
  Path?: string;
  Args?: string[];
  State?: {
    Status: string;
    Running: boolean;
    Paused: boolean;
    Restarting: boolean;
    OOMKilled?: boolean;
    Dead?: boolean;
    Pid?: number;
    ExitCode?: number;
    Error?: string;
    StartedAt?: string;
    FinishedAt?: string;
  };
  Image: string;
  Name: string;
  Config?: {
    Hostname?: string;
    User?: string;
    Env?: string[];
    Cmd?: string[];
    Image?: string;
    WorkingDir?: string;
    Entrypoint?: string[];
    Labels?: Record<string, string>;
  };
  NetworkSettings?: {
    Ports?: Record<string, ContainerPortBinding[] | null>;
    IPAddress?: string;
    Networks?: Record<string, any>;
  };
  Mounts?: ContainerMount[];
  HostConfig?: {
    RestartPolicy?: {
      Name: string;
      MaximumRetryCount?: number;
    };
    PortBindings?: Record<string, ContainerPortBinding[] | null>;
  };
  stack_name?: string;
  service_name?: string;
  source?: string;
  is_self?: boolean;
}
