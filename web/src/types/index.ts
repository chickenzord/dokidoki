export type NodeStatus = 'alive' | 'suspect' | 'offline';

export interface AuthStatus {
  authenticated: boolean;
  auth_required: boolean;
}

export interface Node {
  id: string;
  name: string;
  addresses: string[];
  status: NodeStatus;
  version: string;
  is_self: boolean;
  read_only?: boolean;
  last_seen?: string;
}

export interface ContainerRollup {
  running: number;
  exited: number;
  completed?: number;
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
  pending_import?: boolean;
}

export interface StackDetail extends StackSummary {
  containers: ContainerSummary[];
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
  exit_code?: number;
}

export interface ClusterStack extends StackSummary {
  hostId: string;
  hostName: string;
  hostEndpoint: string;
  hostAddress?: string;
  hostReadOnly?: boolean;
  pending_import?: boolean;
}

export interface ClusterContainer extends ContainerSummary {
  hostId: string;
  hostName: string;
  hostEndpoint: string;
  hostAddress?: string;
  hostReadOnly?: boolean;
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

export interface CreateStackFile {
  name: string;
  content: string;
}

export interface CreateStackRequest {
  name: string;
  files: CreateStackFile[];
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
  read_only?: boolean;
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

export interface OperationResult {
  success: boolean;
  message: string;
  output?: string;
}

export interface UnreachableNode {
  id: string;
  name: string;
  error?: string;
}

export interface ClusterStacksResult {
  items: ClusterStack[];
  unreachableNodes: UnreachableNode[];
}

export interface ClusterContainersResult {
  items: ClusterContainer[];
  unreachableNodes: UnreachableNode[];
}

export interface DockerNetworkIPAMConfig {
  Subnet?: string;
  IPRange?: string;
  Gateway?: string;
  AuxiliaryAddresses?: Record<string, string>;
}

export interface DockerNetworkIPAM {
  Driver?: string;
  Options?: Record<string, string>;
  Config?: DockerNetworkIPAMConfig[];
}

export interface DockerNetworkContainerEndpoint {
  Name?: string;
  EndpointID?: string;
  MacAddress?: string;
  IPv4Address?: string;
  IPv6Address?: string;
}

export interface DockerNetwork {
  Id: string;
  ID?: string;
  Name: string;
  Driver: string;
  Scope: string;
  EnableIPv6: boolean;
  IPAM?: DockerNetworkIPAM;
  Internal: boolean;
  Attachable: boolean;
  Ingress: boolean;
  Containers?: Record<string, DockerNetworkContainerEndpoint>;
  Labels?: Record<string, string>;
  Created?: string;
}

export interface ClusterNetwork extends DockerNetwork {
  hostId: string;
  hostName: string;
  hostEndpoint: string;
  hostAddress?: string;
  hostReadOnly?: boolean;
}

export interface ClusterNetworksResult {
  items: ClusterNetwork[];
  unreachableNodes: UnreachableNode[];
}

export interface DockerVolumeUsageData {
  Size?: number;
  RefCount?: number;
}

export interface DockerVolume {
  Name: string;
  Driver: string;
  Scope: string;
  Mountpoint: string;
  Labels?: Record<string, string>;
  Options?: Record<string, string>;
  CreatedAt?: string;
  Status?: Record<string, any>;
  UsageData?: DockerVolumeUsageData;
}

export interface ClusterVolume extends DockerVolume {
  hostId: string;
  hostName: string;
  hostEndpoint: string;
  hostAddress?: string;
  hostReadOnly?: boolean;
}

export interface ClusterVolumesResult {
  items: ClusterVolume[];
  unreachableNodes: UnreachableNode[];
}

export interface DockerImage {
  Id: string;
  ID?: string;
  RepoTags?: string[];
  RepoDigests?: string[];
  Created: number;
  Size: number;
  VirtualSize?: number;
  SharedSize?: number;
  Labels?: Record<string, string>;
  Containers: number;
}

export interface ClusterImage extends DockerImage {
  hostId: string;
  hostName: string;
  hostEndpoint: string;
  hostAddress?: string;
  hostReadOnly?: boolean;
}

export interface ClusterImagesResult {
  items: ClusterImage[];
  unreachableNodes: UnreachableNode[];
}

export interface DockerEventActor {
  Id?: string;
  ID?: string;
  Attributes?: Record<string, string>;
}

export interface DockerEvent {
  Type: string;
  Action: string;
  Actor: DockerEventActor;
  Time: number;
  TimeNano: number;
  // Node info added when streamed / displayed in cluster view
  hostId?: string;
  hostName?: string;
}
