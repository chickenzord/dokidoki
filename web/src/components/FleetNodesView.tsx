import React from 'react';
import { useNodesQuery } from '../hooks/useFleetData';
import { Badge } from './ui/badge';
import { Server, Radio, Globe, Tag, RefreshCw, AlertCircle, Loader2 } from 'lucide-react';
import { Button } from './ui/button';

export const FleetNodesView: React.FC = () => {
  const { data: nodes = [], isLoading, error, refetch, isFetching } = useNodesQuery();

  const aliveCount = nodes.filter((n) => n.status === 'alive').length;

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-2 border-b border-slate-800">
        <div>
          <h2 className="text-lg font-bold text-slate-100 flex items-center gap-2">
            <Server className="w-5 h-5 text-rose-500" />
            Cluster Peer Nodes
          </h2>
          <p className="text-xs text-slate-400 mt-0.5">
            Decentralized peer-to-peer fleet mesh. All nodes operate as equal cluster members.
          </p>
        </div>

        <div className="flex items-center gap-2">
          <Badge variant="outline" className="text-xs font-mono bg-slate-900 border-slate-700 text-slate-300">
            {aliveCount} / {nodes.length} online
          </Badge>
          <Button
            size="sm"
            variant="ghost"
            onClick={() => refetch()}
            disabled={isLoading || isFetching}
            className="h-8 px-2.5 text-xs text-slate-400 hover:text-white"
          >
            <RefreshCw className={`w-3.5 h-3.5 mr-1 ${isFetching ? 'animate-spin text-rose-500' : ''}`} />
            Refresh
          </Button>
        </div>
      </div>

      {error && (
        <div className="p-4 bg-rose-950/40 border border-rose-800/60 rounded-xl text-rose-300 text-xs flex items-center gap-2">
          <AlertCircle className="w-4 h-4 text-rose-400 flex-shrink-0" />
          <span>Failed to load cluster nodes: {(error as Error).message}</span>
        </div>
      )}

      {isLoading ? (
        <div className="p-12 text-center text-xs text-slate-400 flex flex-col items-center justify-center gap-2">
          <Loader2 className="w-6 h-6 animate-spin text-rose-500" />
          <span>Discovering peer nodes...</span>
        </div>
      ) : nodes.length === 0 ? (
        <div className="p-12 border border-slate-800 rounded-xl text-center text-xs text-slate-500 bg-slate-950/40">
          No peer nodes discovered in the mesh yet.
        </div>
      ) : (
        <div className="border border-slate-800 rounded-xl overflow-hidden bg-slate-950/40">
          <table className="w-full text-left text-xs">
            <thead className="bg-slate-900/80 text-slate-400 font-mono text-[11px] uppercase tracking-wider border-b border-slate-800">
              <tr>
                <th className="p-3.5">Node Name</th>
                <th className="p-3.5">Status</th>
                <th className="p-3.5">Mesh Addresses</th>
                <th className="p-3.5">Dokidoki Version</th>
                <th className="p-3.5">Node ID</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60 font-mono">
              {nodes.map((node) => {
                const isAlive = node.status === 'alive';
                const isSuspect = node.status === 'suspect';

                return (
                  <tr key={node.id} className="hover:bg-slate-800/30 transition-colors">
                    <td className="p-3.5 font-sans font-medium text-slate-200">
                      <div className="flex items-center gap-2">
                        <Radio className={`w-4 h-4 flex-shrink-0 ${isAlive ? 'text-emerald-400' : isSuspect ? 'text-amber-400' : 'text-slate-500'}`} />
                        <span className="font-semibold text-slate-100">{node.name}</span>
                        {node.is_self && (
                          <Badge variant="outline" className="text-[10px] bg-slate-800 text-slate-300 border-slate-700 font-mono">
                            Discovery Origin
                          </Badge>
                        )}
                      </div>
                    </td>

                    <td className="p-3.5">
                      <Badge
                        variant="outline"
                        className={`text-[10px] uppercase font-mono ${
                          isAlive
                            ? 'bg-emerald-950/40 text-emerald-400 border-emerald-800/60'
                            : isSuspect
                            ? 'bg-amber-950/40 text-amber-400 border-amber-800/60'
                            : 'bg-rose-950/40 text-rose-400 border-rose-800/60'
                        }`}
                      >
                        {node.status}
                      </Badge>
                    </td>

                    <td className="p-3.5 text-slate-300">
                      {node.addresses && node.addresses.length > 0 ? (
                        <div className="flex flex-wrap gap-1 items-center">
                          {node.addresses.map((addr, idx) => (
                            <span key={idx} className="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-slate-900 border border-slate-800 text-[11px] text-slate-300">
                              <Globe className="w-3 h-3 text-slate-500" />
                              {addr}
                            </span>
                          ))}
                        </div>
                      ) : (
                        <span className="text-slate-500 italic font-sans text-xs">Direct local socket</span>
                      )}
                    </td>

                    <td className="p-3.5 text-slate-300 text-xs">
                      {node.version ? (
                        <span className="inline-flex items-center gap-1 text-slate-400">
                          <Tag className="w-3 h-3 text-slate-500" />
                          v{node.version}
                        </span>
                      ) : (
                        <span className="text-slate-500">—</span>
                      )}
                    </td>

                    <td className="p-3.5 text-slate-500 text-[11px] truncate max-w-[140px]" title={node.id}>
                      {node.id}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};
