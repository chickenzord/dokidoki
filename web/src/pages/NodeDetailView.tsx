import React from 'react';
import { Link } from '@tanstack/react-router';
import { useNodesQuery } from '../hooks/useClusterData';
import { api } from '../services/api';
import { useQuery } from '@tanstack/react-query';
import { getNodeEndpoint, CLUSTER_NODE_TIMEOUT_MS } from '../hooks/useClusterData';
import { Badge } from '../components/ui/badge';
import { Button } from '../components/ui/button';
import {
  Server,
  Layers,
  Box,
  Radio,
  Cpu,
  HardDrive,
  RefreshCw,
  AlertCircle,
  ArrowLeft,
  Loader2,
} from 'lucide-react';

interface NodeDetailViewProps {
  nodeId: string;
}

export const NodeDetailView: React.FC<NodeDetailViewProps> = ({ nodeId }) => {
  const { data: nodes = [], isLoading: nodesLoading, refetch: refetchNodes } = useNodesQuery();
  const node = nodes.find((n) => n.id === nodeId);

  const endpoint = node ? getNodeEndpoint(node) : '';

  const {
    data: hostInfo,
    isLoading: infoLoading,
    error: infoError,
    refetch: refetchInfo,
    isFetching: infoFetching,
  } = useQuery({
    queryKey: ['node-host-info', nodeId, endpoint],
    queryFn: () => api.getHostInfo(endpoint, { timeout: CLUSTER_NODE_TIMEOUT_MS }),
    enabled: Boolean(node && node.status !== 'offline'),
    refetchInterval: 10000,
  });

  const handleRefresh = () => {
    refetchNodes();
    if (node && node.status !== 'offline') {
      refetchInfo();
    }
  };

  if (nodesLoading) {
    return (
      <div className="p-16 text-center text-xs text-slate-400 flex flex-col items-center justify-center gap-2">
        <Loader2 className="w-6 h-6 animate-spin text-rose-500" />
        <span>Loading node information…</span>
      </div>
    );
  }

  if (!node) {
    return (
      <div className="space-y-4">
        <Link
          to="/nodes"
          className="inline-flex items-center gap-1.5 text-xs text-slate-400 hover:text-white transition-colors"
        >
          <ArrowLeft className="w-3.5 h-3.5" />
          Back to Nodes
        </Link>
        <div className="p-8 border border-slate-800 rounded-xl text-center text-xs text-slate-400 bg-slate-900/40">
          Node with ID <span className="font-mono text-slate-200">{nodeId}</span> not found in cluster topology.
        </div>
      </div>
    );
  }

  const isAlive = node.status === 'alive';
  const isSuspect = node.status === 'suspect';

  return (
    <div className="space-y-6">
      {/* Breadcrumb & Navigation Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-4 border-b border-slate-800">
        <div className="space-y-1">
          <div className="flex items-center gap-2 text-xs text-slate-400">
            <Link to="/nodes" className="hover:text-slate-200 transition-colors">
              Nodes
            </Link>
            <span>/</span>
            <span className="text-slate-200 font-mono">{node.name}</span>
          </div>

          <div className="flex items-center gap-3 pt-1">
            <h1 className="text-xl font-bold text-white flex items-center gap-2">
              <Server className="w-6 h-6 text-emerald-400" />
              {node.name}
            </h1>
            <Badge
              variant="outline"
              className={`text-xs uppercase font-mono ${
                isAlive
                  ? 'bg-emerald-950/40 text-emerald-400 border-emerald-800/60'
                  : isSuspect
                  ? 'bg-amber-950/40 text-amber-400 border-amber-800/60'
                  : 'bg-rose-950/40 text-rose-400 border-rose-800/60'
              }`}
            >
              {node.status}
            </Badge>
            {node.is_self && (
              <Badge variant="outline" className="text-xs bg-slate-800 text-slate-300 border-slate-700">
                Current Origin
              </Badge>
            )}
          </div>
        </div>

        {/* Action tabs to Node-scoped resources */}
        <div className="flex items-center gap-2">
          <Link to="/nodes/$nodeId/stacks" params={{ nodeId: node.id }}>
            <Button
              size="sm"
              variant="outline"
              className="h-8 px-3 text-xs border-slate-800 bg-slate-900 hover:bg-slate-800 text-slate-200 hover:text-white"
            >
              <Layers className="w-3.5 h-3.5 mr-1.5 text-rose-400" />
              Node Stacks
            </Button>
          </Link>

          <Link to="/nodes/$nodeId/containers" params={{ nodeId: node.id }}>
            <Button
              size="sm"
              variant="outline"
              className="h-8 px-3 text-xs border-slate-800 bg-slate-900 hover:bg-slate-800 text-slate-200 hover:text-white"
            >
              <Box className="w-3.5 h-3.5 mr-1.5 text-blue-400" />
              Node Containers
            </Button>
          </Link>

          <Button
            size="sm"
            variant="ghost"
            onClick={handleRefresh}
            disabled={infoFetching}
            className="h-8 px-2.5 text-xs text-slate-400 hover:text-white"
            title="Refresh"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${infoFetching ? 'animate-spin text-rose-500' : ''}`} />
          </Button>
        </div>
      </div>

      {infoError && (
        <div className="p-4 bg-rose-950/40 border border-rose-800/60 rounded-xl text-rose-300 text-xs flex items-center gap-2">
          <AlertCircle className="w-4 h-4 text-rose-400 flex-shrink-0" />
          <span>Failed to query host metrics: {(infoError as Error).message}</span>
        </div>
      )}

      {/* Node & Docker Metrics Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        {/* Node Status Card */}
        <div className="p-4 rounded-xl border border-slate-800 bg-slate-900/40 space-y-2">
          <div className="text-xs font-semibold text-slate-400 uppercase tracking-wider flex items-center gap-1.5">
            <Radio className="w-3.5 h-3.5 text-emerald-400" />
            Cluster Node Info
          </div>
          <div className="space-y-1.5 pt-1 text-xs">
            <div className="flex justify-between items-center">
              <span className="text-slate-400">Node ID:</span>
              <span className="font-mono text-slate-200 truncate max-w-[150px]" title={node.id}>
                {node.id}
              </span>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-slate-400">Version:</span>
              <span className="font-mono text-slate-200">{node.version || 'unknown'}</span>
            </div>
            <div className="flex justify-between items-start">
              <span className="text-slate-400">Addresses:</span>
              <div className="text-right flex flex-col gap-0.5 max-w-[160px]">
                {node.addresses && node.addresses.length > 0 ? (
                  node.addresses.map((a, i) => (
                    <span key={i} className="font-mono text-[11px] text-slate-300 truncate">
                      {a}
                    </span>
                  ))
                ) : (
                  <span className="text-slate-500 italic">Direct socket</span>
                )}
              </div>
            </div>
          </div>
        </div>

        {/* Docker Engine Card */}
        <div className="p-4 rounded-xl border border-slate-800 bg-slate-900/40 space-y-2">
          <div className="text-xs font-semibold text-slate-400 uppercase tracking-wider flex items-center gap-1.5">
            <Cpu className="w-3.5 h-3.5 text-blue-400" />
            Docker Daemon
          </div>
          {infoLoading ? (
            <div className="py-4 text-center text-xs text-slate-500">Loading Docker info…</div>
          ) : hostInfo?.docker ? (
            <div className="space-y-1.5 pt-1 text-xs">
              <div className="flex justify-between items-center">
                <span className="text-slate-400">Engine Version:</span>
                <span className="font-mono text-slate-200">{hostInfo.docker.engineVersion}</span>
              </div>
              <div className="flex justify-between items-center">
                <span className="text-slate-400">API Version:</span>
                <span className="font-mono text-slate-200">{hostInfo.docker.apiVersion}</span>
              </div>
              <div className="flex justify-between items-center">
                <span className="text-slate-400">Architecture:</span>
                <span className="font-mono text-slate-200">{hostInfo.docker.arch}</span>
              </div>
            </div>
          ) : (
            <div className="py-4 text-center text-xs text-slate-500">No Docker daemon info available</div>
          )}
        </div>

        {/* Container Metrics Card */}
        <div className="p-4 rounded-xl border border-slate-800 bg-slate-900/40 space-y-2">
          <div className="text-xs font-semibold text-slate-400 uppercase tracking-wider flex items-center gap-1.5">
            <Box className="w-3.5 h-3.5 text-rose-400" />
            Container Counts
          </div>
          {infoLoading ? (
            <div className="py-4 text-center text-xs text-slate-500">Loading counts…</div>
          ) : hostInfo?.docker ? (
            <div className="space-y-1.5 pt-1 text-xs">
              <div className="flex justify-between items-center">
                <span className="text-slate-400">Total Containers:</span>
                <span className="font-mono font-semibold text-slate-100">{hostInfo.docker.containers}</span>
              </div>
              <div className="flex justify-between items-center">
                <span className="text-slate-400">Running:</span>
                <span className="font-mono font-semibold text-emerald-400">{hostInfo.docker.containersRunning}</span>
              </div>
              <div className="flex justify-between items-center">
                <span className="text-slate-400">Stopped / Paused:</span>
                <span className="font-mono text-slate-300">
                  {hostInfo.docker.containersStopped} / {hostInfo.docker.containersPaused}
                </span>
              </div>
            </div>
          ) : (
            <div className="py-4 text-center text-xs text-slate-500">—</div>
          )}
        </div>

        {/* Host OS Card */}
        <div className="p-4 rounded-xl border border-slate-800 bg-slate-900/40 space-y-2">
          <div className="text-xs font-semibold text-slate-400 uppercase tracking-wider flex items-center gap-1.5">
            <HardDrive className="w-3.5 h-3.5 text-purple-400" />
            Host OS & Storage
          </div>
          {infoLoading ? (
            <div className="py-4 text-center text-xs text-slate-500">Loading host info…</div>
          ) : hostInfo ? (
            <div className="space-y-1.5 pt-1 text-xs">
              <div className="flex justify-between items-center">
                <span className="text-slate-400">OS:</span>
                <span className="font-mono text-slate-200">{hostInfo.os || hostInfo.docker?.os}</span>
              </div>
              <div className="flex justify-between items-center">
                <span className="text-slate-400">Hostname:</span>
                <span className="font-mono text-slate-200 truncate max-w-[140px]" title={hostInfo.hostname}>
                  {hostInfo.hostname}
                </span>
              </div>
              <div className="flex justify-between items-center">
                <span className="text-slate-400">Stacks Dir:</span>
                <span className="font-mono text-[11px] text-slate-300 truncate max-w-[140px]" title={hostInfo.stacksDir}>
                  {hostInfo.stacksDir || '—'}
                </span>
              </div>
            </div>
          ) : (
            <div className="py-4 text-center text-xs text-slate-500">—</div>
          )}
        </div>
      </div>
    </div>
  );
};
