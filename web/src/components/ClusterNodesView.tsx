import React, { useState } from 'react';
import { useNodesQuery, useDeleteNodeMutation, useAddNodeMutation } from '../hooks/useClusterData';
import { Badge } from './ui/badge';
import { Server, Radio, Globe, Tag, RefreshCw, AlertCircle, Loader2, Trash2, LogOut, Plus } from 'lucide-react';
import { Button } from './ui/button';
import { Input } from './ui/input';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from './ui/dialog';
import { Node } from '../types';

export const ClusterNodesView: React.FC = () => {
  const { data: nodes = [], isLoading, error, refetch, isFetching } = useNodesQuery();
  const deleteMutation = useDeleteNodeMutation();
  const addMutation = useAddNodeMutation();

  const [targetNode, setTargetNode] = useState<Node | null>(null);
  const [isAddOpen, setIsAddOpen] = useState(false);
  const [nodeAddress, setNodeAddress] = useState('');
  const [addError, setAddError] = useState<string | null>(null);

  const aliveCount = nodes.filter((n) => n.status === 'alive').length;

  const handleOpenAdd = () => {
    setNodeAddress('');
    setAddError(null);
    setIsAddOpen(true);
  };

  const handleAddNode = async (e: React.FormEvent) => {
    e.preventDefault();
    const addr = nodeAddress.trim();
    if (!addr) {
      setAddError('Please enter a host or URL');
      return;
    }
    setAddError(null);
    try {
      await addMutation.mutateAsync(addr);
      setIsAddOpen(false);
      setNodeAddress('');
    } catch (err: any) {
      const msg = err.message || 'Connection failed or invalid cluster token';
      setAddError(msg);
    }
  };

  const handleConfirmAction = async () => {
    if (!targetNode) return;
    try {
      await deleteMutation.mutateAsync(targetNode.id);
      setTargetNode(null);
    } catch (e) {
      console.error('Failed to perform cluster node action:', e);
    }
  };

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-2 border-b border-slate-800">
        <div>
          <h2 className="text-lg font-bold text-slate-100 flex items-center gap-2">
            <Server className="w-5 h-5 text-rose-500" />
            Cluster Nodes
          </h2>
          <p className="text-xs text-slate-400 mt-0.5">
            All nodes operate as equal cluster members.
          </p>
        </div>

        <div className="flex items-center gap-2">
          <Badge variant="outline" className="text-xs font-mono bg-slate-900 border-slate-700 text-slate-300">
            {aliveCount} / {nodes.length} online
          </Badge>
          <Button
            size="sm"
            variant="outline"
            onClick={handleOpenAdd}
            className="h-8 px-2.5 text-xs border-slate-700 bg-slate-900 hover:bg-slate-800 text-slate-200 hover:text-white"
          >
            <Plus className="w-3.5 h-3.5 mr-1 text-rose-500" />
            Add Node
          </Button>
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
          <span>Discovering nodes...</span>
        </div>
      ) : nodes.length === 0 ? (
        <div className="p-12 border border-slate-800 rounded-xl text-center text-xs text-slate-500 bg-slate-950/40">
          No nodes discovered in the cluster yet.
        </div>
      ) : (
        <div className="border border-slate-800 rounded-xl overflow-hidden bg-slate-950/40">
          <table className="w-full text-left text-xs">
            <thead className="bg-slate-900/80 text-slate-400 font-mono text-[11px] uppercase tracking-wider border-b border-slate-800">
              <tr>
                <th className="p-3.5">Node Name</th>
                <th className="p-3.5">Status</th>
                <th className="p-3.5">Addresses</th>
                <th className="p-3.5">Dokidoki Version</th>
                <th className="p-3.5">Node ID</th>
                <th className="p-3.5 text-right">Actions</th>
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
                          {node.version}
                        </span>
                      ) : (
                        <span className="text-slate-500">—</span>
                      )}
                    </td>

                    <td className="p-3.5 text-slate-500 text-[11px] truncate max-w-[140px]" title={node.id}>
                      {node.id}
                    </td>

                    <td className="p-3.5 text-right font-sans">
                      {node.is_self ? (
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => setTargetNode(node)}
                          className="h-7 px-2 text-xs text-amber-400/80 hover:text-amber-300 hover:bg-amber-950/30"
                          title="Leave Cluster"
                        >
                          <LogOut className="w-3.5 h-3.5 mr-1" />
                          Leave Cluster
                        </Button>
                      ) : (
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => setTargetNode(node)}
                          className="h-7 px-2 text-xs text-rose-400/80 hover:text-rose-300 hover:bg-rose-950/30"
                          title="Delete Node"
                        >
                          <Trash2 className="w-3.5 h-3.5 mr-1" />
                          Delete
                        </Button>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* Confirmation Dialog */}
      <Dialog open={targetNode !== null} onOpenChange={(open) => !open && setTargetNode(null)}>
        <DialogContent className="sm:max-w-md bg-slate-900 border-slate-800 text-slate-100">
          <DialogHeader>
            <DialogTitle className="text-base font-semibold">
              {targetNode?.is_self ? 'Leave Cluster?' : 'Remove Node from Cluster?'}
            </DialogTitle>
            <DialogDescription className="text-xs text-slate-400 mt-2">
              {targetNode?.is_self ? (
                <>
                  This will detach this node (<span className="text-slate-200 font-mono font-medium">{targetNode?.name}</span>) from the cluster and reset it to standalone mode. Local stacks remain unaffected.
                </>
              ) : (
                <>
                  Are you sure you want to remove node{' '}
                  <span className="text-slate-200 font-mono font-medium">{targetNode?.name}</span> ({targetNode?.id}) from the cluster? It will be marked with a tombstone to prevent resurrection.
                </>
              )}
            </DialogDescription>
          </DialogHeader>

          {deleteMutation.isError && (
            <div className="p-3 bg-rose-950/50 border border-rose-800 rounded text-rose-300 text-xs">
              {(deleteMutation.error as Error)?.message || 'Action failed'}
            </div>
          )}

          <DialogFooter className="mt-4 gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setTargetNode(null)}
              disabled={deleteMutation.isPending}
              className="border-slate-700 text-slate-300 hover:bg-slate-800"
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              size="sm"
              onClick={handleConfirmAction}
              disabled={deleteMutation.isPending}
              className={
                targetNode?.is_self
                  ? 'bg-amber-600 hover:bg-amber-500 text-white border-0'
                  : 'bg-rose-600 hover:bg-rose-500 text-white border-0'
              }
            >
              {deleteMutation.isPending ? (
                <Loader2 className="w-3.5 h-3.5 animate-spin mr-1" />
              ) : targetNode?.is_self ? (
                <LogOut className="w-3.5 h-3.5 mr-1" />
              ) : (
                <Trash2 className="w-3.5 h-3.5 mr-1" />
              )}
              {targetNode?.is_self ? 'Confirm Leave' : 'Remove Node'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Add Node Dialog */}
      <Dialog open={isAddOpen} onOpenChange={(open) => !open && !addMutation.isPending && setIsAddOpen(false)}>
        <DialogContent className="sm:max-w-md bg-slate-900 border-slate-800 text-slate-100">
          <form onSubmit={handleAddNode}>
            <DialogHeader>
              <DialogTitle className="text-base font-semibold flex items-center gap-2">
                <Plus className="w-4 h-4 text-rose-500" />
                Add Cluster Node
              </DialogTitle>
              <DialogDescription className="text-xs text-slate-400 mt-2">
                Enter the IP address or hostname of another Dokidoki node. If it shares your cluster token, both nodes will pair immediately and exchange cluster topologies.
              </DialogDescription>
            </DialogHeader>

            <div className="py-4 space-y-3">
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1.5">
                  Node URL or Host:Port
                </label>
                <Input
                  type="text"
                  placeholder="e.g. 192.168.1.50:8080 or http://192.168.1.50:8080"
                  value={nodeAddress}
                  onChange={(e) => {
                    setNodeAddress(e.target.value);
                    if (addError) setAddError(null);
                  }}
                  disabled={addMutation.isPending}
                  className="bg-slate-950 border-slate-700 text-slate-200 text-xs font-mono focus-visible:ring-rose-500"
                  autoFocus
                />
              </div>

              {addError && (
                <div className="p-3 bg-rose-950/60 border border-rose-800 rounded-md text-rose-300 text-xs flex items-center gap-2">
                  <AlertCircle className="w-4 h-4 text-rose-400 flex-shrink-0" />
                  <span>{addError}</span>
                </div>
              )}
            </div>

            <DialogFooter className="mt-2 gap-2">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => setIsAddOpen(false)}
                disabled={addMutation.isPending}
                className="border-slate-700 text-slate-300 hover:bg-slate-800"
              >
                Cancel
              </Button>
              <Button
                type="submit"
                size="sm"
                disabled={addMutation.isPending || !nodeAddress.trim()}
                className="bg-rose-600 hover:bg-rose-500 text-white border-0"
              >
                {addMutation.isPending ? (
                  <>
                    <Loader2 className="w-3.5 h-3.5 animate-spin mr-1.5" />
                    Connecting...
                  </>
                ) : (
                  'Connect Node'
                )}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  );
};
