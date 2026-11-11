import React from 'react';
import { useParams, Link } from '@tanstack/react-router';
import { useNodesQuery, useStackDetailQuery, getNodeEndpoint } from '../hooks/useClusterData';
import { StackDetailSheet } from '../components/StackDetailSheet';
import { ContainerDetailSheet } from '../components/ContainerDetailSheet';
import { ClusterStack, ClusterContainer } from '../types';
import { Loader2, AlertCircle, ArrowLeft } from 'lucide-react';

export const NodeStackDetailRouteComponent: React.FC = () => {
  const params = useParams({ strict: false }) as { nodeId: string; stackName: string };
  const { nodeId, stackName } = params;

  const { data: nodes = [], isLoading: nodesLoading } = useNodesQuery();
  const node = nodes.find((n) => n.id === nodeId);
  const endpoint = node ? getNodeEndpoint(node) : (nodeId === 'local' ? '' : `/api/v1/nodes/${encodeURIComponent(nodeId)}/proxy`);

  const { data: stackDetail, isLoading: stackLoading, error } = useStackDetailQuery(
    endpoint,
    stackName,
    Boolean(stackName)
  );

  const [selectedContainer, setSelectedContainer] = React.useState<ClusterContainer | null>(null);

  const clusterStack: ClusterStack | null = stackDetail
    ? {
        ...stackDetail,
        hostId: node?.id || nodeId,
        hostName: node?.name || nodeId,
        hostEndpoint: endpoint,
        hostAddress: node?.addresses?.[0],
      }
    : null;

  const handleClose = () => {
    // Navigate back to node stacks list
    window.history.back();
  };

  if (nodesLoading || (stackLoading && !stackDetail)) {
    return (
      <div className="p-16 text-center text-xs text-slate-400 flex flex-col items-center justify-center gap-2">
        <Loader2 className="w-6 h-6 animate-spin text-rose-500" />
        <span>Loading stack detail…</span>
      </div>
    );
  }

  if (error || !clusterStack) {
    return (
      <div className="space-y-4">
        <Link
          to="/nodes/$nodeId/stacks"
          params={{ nodeId }}
          className="inline-flex items-center gap-1.5 text-xs text-slate-400 hover:text-white transition-colors"
        >
          <ArrowLeft className="w-3.5 h-3.5" />
          Back to Node Stacks
        </Link>
        <div className="p-8 border border-slate-800 rounded-xl text-center text-xs text-slate-400 bg-slate-900/40">
          <AlertCircle className="w-5 h-5 text-rose-400 mx-auto mb-2" />
          Failed to load stack <span className="font-mono text-slate-200">{stackName}</span> on node{' '}
          <span className="font-mono text-slate-200">{node?.name || nodeId}</span>.
        </div>
      </div>
    );
  }

  return (
    <div>
      <StackDetailSheet
        stack={clusterStack}
        isOpen={true}
        onClose={handleClose}
        onSelectContainer={(containerId, hostEndpoint, containerSummary) => {
          setSelectedContainer({
            id: containerId,
            name: containerSummary?.name || containerId.substring(0, 12),
            image: containerSummary?.image || '',
            state: containerSummary?.state || 'running',
            status: containerSummary?.status || '',
            created: containerSummary?.created || 0,
            ports: containerSummary?.ports || [],
            labels: containerSummary?.labels || {},
            hostId: node?.id || nodeId,
            hostName: node?.name || nodeId,
            hostEndpoint,
            stack: stackName,
            service: containerSummary?.service,
          });
        }}
      />

      <ContainerDetailSheet
        container={selectedContainer}
        isOpen={Boolean(selectedContainer)}
        onClose={() => setSelectedContainer(null)}
      />
    </div>
  );
};
