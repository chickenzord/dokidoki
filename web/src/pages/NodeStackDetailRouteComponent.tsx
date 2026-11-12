import React from 'react';
import { useParams } from '@tanstack/react-router';
import { useNodesQuery, useStackDetailQuery, getNodeEndpoint } from '../hooks/useClusterData';
import { useDocumentTitle } from '../hooks/useDocumentTitle';
import { StackDetailSheet } from '../components/StackDetailSheet';
import { BreadcrumbNav } from '../components/BreadcrumbNav';
import { ClusterStack } from '../types';
import { Loader2, AlertCircle, ArrowLeft, Layers, Server } from 'lucide-react';
import { useSmartBack } from '../lib/navigation';

export const NodeStackDetailRouteComponent: React.FC = () => {
  const params = useParams({ strict: false }) as { nodeId: string; stackName: string };
  const { nodeId, stackName } = params;
  const { goBackOr } = useSmartBack();

  const { data: nodes = [], isLoading: nodesLoading } = useNodesQuery();
  const node = nodes.find((n) => n.id === nodeId);
  const nodeLabel = node?.name || (nodeId === 'local' ? 'Local' : (nodeId ? nodeId.substring(0, 8) : ''));
  useDocumentTitle(`Stack: ${stackName}${nodeLabel ? ` (${nodeLabel})` : ''}`);
  const endpoint = node ? getNodeEndpoint(node) : (nodeId === 'local' ? '' : `/api/v1/nodes/${encodeURIComponent(nodeId)}/proxy`);

  const { data: stackDetail, isLoading: stackLoading, error } = useStackDetailQuery(
    endpoint,
    stackName,
    Boolean(stackName)
  );

  const clusterStack: ClusterStack | null = stackDetail
    ? {
        ...stackDetail,
        hostId: node?.id || nodeId,
        hostName: node?.name || nodeId,
        hostEndpoint: endpoint,
        hostAddress: node?.addresses?.[0],
        hostReadOnly: node?.read_only,
      }
    : null;

  const handleBack = () => {
    goBackOr({ to: '/nodes/$nodeId/stacks', params: { nodeId } });
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
        <button
          type="button"
          onClick={handleBack}
          className="inline-flex items-center gap-1.5 text-xs text-slate-400 hover:text-white transition-colors cursor-pointer"
        >
          <ArrowLeft className="w-3.5 h-3.5" />
          Back to Node Stacks
        </button>
        <div className="p-8 border border-slate-800 rounded-xl text-center text-xs text-slate-400 bg-slate-900/40">
          <AlertCircle className="w-5 h-5 text-rose-400 mx-auto mb-2" />
          Failed to load stack <span className="font-mono text-slate-200">{stackName}</span> on node{' '}
          <span className="font-mono text-slate-200">{node?.name || nodeId}</span>.
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-4 max-w-7xl mx-auto">
      {/* Top Breadcrumbs: direct hierarchy node > stack */}
      <BreadcrumbNav
        items={[
          {
            label: node?.name || nodeId,
            to: '/nodes/$nodeId',
            params: { nodeId },
            icon: Server,
          },
          {
            label: clusterStack.name,
            icon: Layers,
          },
        ]}
      />

      {/* Standalone Stack Detail View */}
      <div className="pt-2">
        <StackDetailSheet
          stack={clusterStack}
          isStandalonePage={true}
          isOpen={true}
        />
      </div>
    </div>
  );
};
