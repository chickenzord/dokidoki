import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../services/api';
import {
  Node,
  ClusterStack,
  StackDetail,
  ClusterContainer,
  CreateStackRequest,
  StackFilesResponse,
  EnrichedContainerInspect,
  ContainerComposeResponse,
} from '../types';

export function getNodeEndpoint(node: Node): string {
  if (node.is_self) {
    return '';
  }
  const addr = node.addresses?.[0];
  if (!addr) return '';
  if (addr.startsWith('http://') || addr.startsWith('https://')) {
    return addr;
  }
  return `http://${addr}`;
}

export const CLUSTER_NODE_TIMEOUT_MS = 3500;

export function useNodesQuery() {
  return useQuery({
    queryKey: ['nodes'],
    queryFn: () => api.getNodes(undefined, { timeout: CLUSTER_NODE_TIMEOUT_MS }),
    refetchInterval: 10000,
  });
}

export function useClusterStacksQuery(selectedHostId?: string | null) {
  const { data: nodes = [], isLoading: nodesLoading } = useNodesQuery();

  return useQuery({
    queryKey: ['cluster-stacks', selectedHostId, nodes.map(n => `${n.id}:${n.status}`).join(',')],
    queryFn: async (): Promise<ClusterStack[]> => {
      const isAllNodes = !selectedHostId || selectedHostId === 'all';

      // When querying for all nodes, filter out offline nodes to avoid stalling on dead peers
      const filteredNodes = isAllNodes
        ? nodes.filter((n) => n.status !== 'offline')
        : nodes.filter((n) => n.id === selectedHostId);

      // If nodes list is empty or no valid nodes match, fall back to self if available, or bootstrap local host
      const targetNodes = filteredNodes.length > 0
        ? filteredNodes
        : (isAllNodes && nodes.some((n) => n.is_self))
          ? nodes.filter((n) => n.is_self)
          : [{ id: 'local', name: 'Local Host', addresses: [], status: 'alive' as const, version: '', is_self: true }];

      const results = await Promise.allSettled(
        targetNodes.map(async (node) => {
          // If remote node has no reachable address, do not attempt query or fallback to local
          if (!node.is_self && (!node.addresses || node.addresses.length === 0)) {
            return [];
          }
          const endpoint = getNodeEndpoint(node);
          const stacks = await api.getStacks(endpoint, undefined, { timeout: CLUSTER_NODE_TIMEOUT_MS });
          return stacks.map((s) => ({
            ...s,
            hostId: node.id,
            hostName: node.name,
            hostEndpoint: endpoint,
          }));
        })
      );

      const allStacks: ClusterStack[] = [];
      results.forEach((res) => {
        if (res.status === 'fulfilled') {
          allStacks.push(...res.value);
        } else {
          console.warn('Failed to fetch stacks for node:', res.reason);
        }
      });

      return allStacks;
    },
    enabled: !nodesLoading,
    refetchInterval: 10000,
  });
}

export function useClusterContainersQuery(selectedHostId?: string | null) {
  const { data: nodes = [], isLoading: nodesLoading } = useNodesQuery();

  return useQuery({
    queryKey: ['cluster-containers', selectedHostId, nodes.map(n => `${n.id}:${n.status}`).join(',')],
    queryFn: async (): Promise<ClusterContainer[]> => {
      const isAllNodes = !selectedHostId || selectedHostId === 'all';

      // When querying for all nodes, filter out offline nodes to avoid stalling on dead peers
      const filteredNodes = isAllNodes
        ? nodes.filter((n) => n.status !== 'offline')
        : nodes.filter((n) => n.id === selectedHostId);

      // If nodes list is empty or no valid nodes match, fall back to self if available, or bootstrap local host
      const targetNodes = filteredNodes.length > 0
        ? filteredNodes
        : (isAllNodes && nodes.some((n) => n.is_self))
          ? nodes.filter((n) => n.is_self)
          : [{ id: 'local', name: 'Local Host', addresses: [], status: 'alive' as const, version: '', is_self: true }];

      const results = await Promise.allSettled(
        targetNodes.map(async (node) => {
          // If remote node has no reachable address, do not attempt query or fallback to local
          if (!node.is_self && (!node.addresses || node.addresses.length === 0)) {
            return [];
          }
          const endpoint = getNodeEndpoint(node);
          const containers = await api.getContainers(endpoint, undefined, { timeout: CLUSTER_NODE_TIMEOUT_MS });
          return containers.map((c) => ({
            ...c,
            hostId: node.id,
            hostName: node.name,
            hostEndpoint: endpoint,
          }));
        })
      );

      const allContainers: ClusterContainer[] = [];
      results.forEach((res) => {
        if (res.status === 'fulfilled') {
          allContainers.push(...res.value);
        } else {
          console.warn('Failed to fetch containers for node:', res.reason);
        }
      });

      return allContainers;
    },
    enabled: !nodesLoading,
    refetchInterval: 10000,
  });
}

export function useStackDetailQuery(hostEndpoint: string, stackName: string, enabled = true) {
  return useQuery<StackDetail>({
    queryKey: ['stack-detail', hostEndpoint, stackName],
    queryFn: () => api.getStack(stackName, hostEndpoint),
    enabled: Boolean(stackName) && enabled,
    refetchInterval: 5000,
  });
}

export function useStackFilesQuery(
  hostEndpoint: string,
  stackName: string,
  options?: { enabled?: boolean; read?: boolean }
) {
  const isEnabled = (options?.enabled !== undefined ? options.enabled : true) && Boolean(stackName);
  return useQuery({
    queryKey: ['stack-files', hostEndpoint, stackName, options?.read ?? false],
    queryFn: async (): Promise<StackFilesResponse> => {
      return api.getStackFiles(stackName, options?.read, hostEndpoint);
    },
    enabled: isEnabled,
  });
}

export function useReadStackFilesMutation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({
      stackName,
      hostEndpoint,
    }: {
      stackName: string;
      hostEndpoint?: string;
    }) => {
      return api.getStackFiles(stackName, true, hostEndpoint);
    },
    onSuccess: (data, variables) => {
      queryClient.setQueryData(
        ['stack-files', variables.hostEndpoint || '', variables.stackName, true],
        data
      );
    },
  });
}

export function useContainerInspectQuery(hostEndpoint: string, containerId: string) {
  return useQuery({
    queryKey: ['container-inspect', hostEndpoint, containerId],
    queryFn: async (): Promise<EnrichedContainerInspect> => {
      return api.inspectContainer(containerId, hostEndpoint);
    },
    enabled: Boolean(containerId),
  });
}

export function useContainerComposeQuery(hostEndpoint: string, containerId: string) {
  return useQuery({
    queryKey: ['container-compose', hostEndpoint, containerId],
    queryFn: async (): Promise<ContainerComposeResponse> => {
      return api.getContainerCompose(containerId, hostEndpoint);
    },
    enabled: Boolean(containerId),
  });
}

export function useCreateStackMutation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ data, hostEndpoint }: { data: CreateStackRequest; hostEndpoint?: string }) => {
      return api.createStack(data, hostEndpoint);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['cluster-stacks'] });
      queryClient.invalidateQueries({ queryKey: ['cluster-containers'] });
      queryClient.invalidateQueries({ queryKey: ['stack-files'] });
    },
  });
}

export function useUpdateStackMutation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({
      name,
      data,
      hostEndpoint,
    }: {
      name: string;
      data: CreateStackRequest;
      hostEndpoint?: string;
    }) => {
      return api.updateStack(name, data, hostEndpoint);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['cluster-stacks'] });
      queryClient.invalidateQueries({ queryKey: ['cluster-containers'] });
      queryClient.invalidateQueries({ queryKey: ['stack-files'] });
    },
  });
}

export function useStackContainersQuery(hostEndpoint: string, stackName: string) {
  return useQuery({
    queryKey: ['stack-containers', hostEndpoint, stackName],
    queryFn: async (): Promise<ClusterContainer[]> => {
      const containers = await api.getContainers(hostEndpoint, stackName);
      return containers.map((c) => ({
        ...c,
        hostId: '',
        hostName: '',
        hostEndpoint,
      }));
    },
    enabled: Boolean(stackName),
    refetchInterval: 10000,
  });
}
