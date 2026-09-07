import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../services/api';
import {
  Node,
  FleetStack,
  FleetContainer,
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

export function useNodesQuery() {
  return useQuery({
    queryKey: ['nodes'],
    queryFn: () => api.getNodes(),
    refetchInterval: 10000,
  });
}

export function useFleetStacksQuery(selectedHostId?: string | null) {
  const { data: nodes = [], isLoading: nodesLoading } = useNodesQuery();

  return useQuery({
    queryKey: ['fleet-stacks', selectedHostId, nodes.map(n => `${n.id}:${n.status}`).join(',')],
    queryFn: async (): Promise<FleetStack[]> => {
      // If nodes list is empty, try querying the local endpoint as bootstrap
      const targetNodes = nodes.length > 0
        ? (selectedHostId && selectedHostId !== 'all' ? nodes.filter(n => n.id === selectedHostId) : nodes)
        : [{ id: 'local', name: 'Local Host', addresses: [], status: 'alive' as const, version: '', is_self: true }];

      const results = await Promise.allSettled(
        targetNodes.map(async (node) => {
          const endpoint = getNodeEndpoint(node);
          const stacks = await api.getStacks(endpoint);
          return stacks.map((s) => ({
            ...s,
            hostId: node.id,
            hostName: node.name,
            hostEndpoint: endpoint,
          }));
        })
      );

      const allStacks: FleetStack[] = [];
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

export function useFleetContainersQuery(selectedHostId?: string | null) {
  const { data: nodes = [], isLoading: nodesLoading } = useNodesQuery();

  return useQuery({
    queryKey: ['fleet-containers', selectedHostId, nodes.map(n => `${n.id}:${n.status}`).join(',')],
    queryFn: async (): Promise<FleetContainer[]> => {
      const targetNodes = nodes.length > 0
        ? (selectedHostId && selectedHostId !== 'all' ? nodes.filter(n => n.id === selectedHostId) : nodes)
        : [{ id: 'local', name: 'Local Host', addresses: [], status: 'alive' as const, version: '', is_self: true }];

      const results = await Promise.allSettled(
        targetNodes.map(async (node) => {
          const endpoint = getNodeEndpoint(node);
          const containers = await api.getContainers(endpoint);
          return containers.map((c) => ({
            ...c,
            hostId: node.id,
            hostName: node.name,
            hostEndpoint: endpoint,
          }));
        })
      );

      const allContainers: FleetContainer[] = [];
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

export function useStackFilesQuery(hostEndpoint: string, stackName: string) {
  return useQuery({
    queryKey: ['stack-files', hostEndpoint, stackName],
    queryFn: async (): Promise<StackFilesResponse> => {
      return api.getStackFiles(stackName, hostEndpoint);
    },
    enabled: Boolean(stackName),
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
      queryClient.invalidateQueries({ queryKey: ['fleet-stacks'] });
      queryClient.invalidateQueries({ queryKey: ['fleet-containers'] });
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
      data: Partial<CreateStackRequest>;
      hostEndpoint?: string;
    }) => {
      return api.updateStack(name, data, hostEndpoint);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['fleet-stacks'] });
      queryClient.invalidateQueries({ queryKey: ['fleet-containers'] });
      queryClient.invalidateQueries({ queryKey: ['stack-files'] });
    },
  });
}

export function useStackContainersQuery(hostEndpoint: string, stackName: string) {
  return useQuery({
    queryKey: ['stack-containers', hostEndpoint, stackName],
    queryFn: async (): Promise<FleetContainer[]> => {
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
