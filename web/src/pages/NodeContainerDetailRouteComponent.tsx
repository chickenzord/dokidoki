import React from 'react';
import { useParams, Link } from '@tanstack/react-router';
import { useNodesQuery, useContainerInspectQuery, getNodeEndpoint } from '../hooks/useClusterData';
import { ContainerDetailSheet } from '../components/ContainerDetailSheet';
import { ClusterContainer } from '../types';
import { Loader2, AlertCircle, ArrowLeft } from 'lucide-react';

export const NodeContainerDetailRouteComponent: React.FC = () => {
  const params = useParams({ strict: false }) as { nodeId: string; containerId: string };
  const { nodeId, containerId } = params;

  const { data: nodes = [], isLoading: nodesLoading } = useNodesQuery();
  const node = nodes.find((n) => n.id === nodeId);
  const endpoint = node ? getNodeEndpoint(node) : (nodeId === 'local' ? '' : `/api/v1/nodes/${encodeURIComponent(nodeId)}/proxy`);

  const { data: inspectData, isLoading: inspectLoading, error } = useContainerInspectQuery(
    endpoint,
    containerId
  );

  const clusterContainer: ClusterContainer | null = inspectData
    ? {
        id: inspectData.Id || containerId,
        name: inspectData.Name || containerId.substring(0, 12),
        image: inspectData.Config?.Image || '',
        state: inspectData.State?.Running ? 'running' : inspectData.State?.Paused ? 'paused' : 'exited',
        status: inspectData.State?.Status || '',
        created: inspectData.Created ? Math.floor(new Date(inspectData.Created).getTime() / 1000) : 0,
        ports: Object.entries(inspectData.NetworkSettings?.Ports || {}).flatMap(([portProto, bindings]) => {
          const [portStr, type] = portProto.split('/');
          const privatePort = parseInt(portStr, 10);
          if (!bindings || bindings.length === 0) {
            return [{ privatePort, type }];
          }
          return bindings.map((b) => ({
            privatePort,
            publicPort: b.HostPort ? parseInt(b.HostPort, 10) : undefined,
            ip: b.HostIp,
            type,
          }));
        }),
        labels: inspectData.Config?.Labels || {},
        hostId: node?.id || nodeId,
        hostName: node?.name || nodeId,
        hostEndpoint: endpoint,
        hostAddress: node?.addresses?.[0],
        hostReadOnly: node?.read_only,
        stack: inspectData.stack_name || inspectData.Config?.Labels?.['com.docker.compose.project'],
        service: inspectData.Config?.Labels?.['com.docker.compose.service'],
        exit_code: inspectData.State?.ExitCode,
      }
    : null;

  const handleClose = () => {
    // Navigate back to node containers list
    window.history.back();
  };

  if (nodesLoading || (inspectLoading && !inspectData)) {
    return (
      <div className="p-16 text-center text-xs text-slate-400 flex flex-col items-center justify-center gap-2">
        <Loader2 className="w-6 h-6 animate-spin text-rose-500" />
        <span>Loading container detail…</span>
      </div>
    );
  }

  if (error || !clusterContainer) {
    return (
      <div className="space-y-4">
        <Link
          to="/nodes/$nodeId/containers"
          params={{ nodeId }}
          className="inline-flex items-center gap-1.5 text-xs text-slate-400 hover:text-white transition-colors"
        >
          <ArrowLeft className="w-3.5 h-3.5" />
          Back to Node Containers
        </Link>
        <div className="p-8 border border-slate-800 rounded-xl text-center text-xs text-slate-400 bg-slate-900/40">
          <AlertCircle className="w-5 h-5 text-rose-400 mx-auto mb-2" />
          Failed to load container <span className="font-mono text-slate-200">{containerId}</span> on node{' '}
          <span className="font-mono text-slate-200">{node?.name || nodeId}</span>.
        </div>
      </div>
    );
  }

  return (
    <ContainerDetailSheet
      container={clusterContainer}
      isOpen={true}
      onClose={handleClose}
    />
  );
};
