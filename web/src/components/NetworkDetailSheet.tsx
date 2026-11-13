import React from 'react';
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from './ui/sheet';
import { Badge } from './ui/badge';
import { Button } from './ui/button';
import { ScrollArea } from './ui/scroll-area';
import { useNetworkDetailQuery } from '../hooks/useClusterData';
import { Network, Server, Globe, Shield, RefreshCw, Loader2, Box } from 'lucide-react';
import { ClusterNetwork } from '../types';

interface NetworkDetailSheetProps {
  network: ClusterNetwork | null;
  isOpen: boolean;
  onClose: () => void;
}

export const NetworkDetailSheet: React.FC<NetworkDetailSheetProps> = ({
  network,
  isOpen,
  onClose,
}) => {
  const netId = network?.Id || network?.ID || '';
  const { data: detailData, isLoading, refetch, isFetching } = useNetworkDetailQuery(
    network?.hostEndpoint || '',
    netId,
    isOpen && Boolean(netId)
  );

  if (!network) return null;

  const currentNetwork = detailData || network;
  const currentNetId = currentNetwork.Id || currentNetwork.ID || netId;
  const ipamConfigs = currentNetwork.IPAM?.Config || [];
  const containers = currentNetwork.Containers ? Object.entries(currentNetwork.Containers) : [];

  return (
    <Sheet open={isOpen} onOpenChange={(open) => !open && onClose()}>
      <SheetContent className="w-full sm:max-w-2xl bg-slate-950 border-slate-800 text-slate-100 p-0 flex flex-col">
        <SheetHeader className="p-6 border-b border-slate-800 flex-shrink-0">
          <div className="flex items-center justify-between gap-3">
            <div className="flex items-center gap-2.5 min-w-0">
              <div className="p-2 rounded-lg bg-slate-900 border border-slate-800 text-slate-300">
                <Network className="w-5 h-5 text-slate-400" />
              </div>
              <div className="min-w-0">
                <SheetTitle className="text-lg font-bold text-slate-100 truncate flex items-center gap-2">
                  <span>{currentNetwork.Name}</span>
                </SheetTitle>
                <SheetDescription className="text-xs text-slate-400 font-mono truncate">
                  ID: {currentNetId}
                </SheetDescription>
              </div>
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={() => refetch()}
              disabled={isFetching}
              className="h-8 px-2.5 border-slate-800 bg-slate-900 hover:bg-slate-800 text-slate-300"
            >
              <RefreshCw className={`w-3.5 h-3.5 mr-1.5 ${isFetching ? 'animate-spin' : ''}`} />
              Refresh
            </Button>
          </div>
        </SheetHeader>

        <ScrollArea className="flex-1 p-6">
          {isLoading && !detailData ? (
            <div className="flex items-center justify-center py-12">
              <Loader2 className="w-6 h-6 animate-spin text-slate-400" />
            </div>
          ) : (
            <div className="space-y-6">
              {/* Node / Scope summary */}
              <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
                <div className="p-3 rounded-lg bg-slate-900/60 border border-slate-800/80">
                  <div className="text-[11px] text-slate-400 mb-1 flex items-center gap-1.5">
                    <Server className="w-3.5 h-3.5 text-slate-400" />
                    Host Node
                  </div>
                  <div className="text-xs font-semibold text-slate-200 truncate">{network.hostName}</div>
                </div>

                <div className="p-3 rounded-lg bg-slate-900/60 border border-slate-800/80">
                  <div className="text-[11px] text-slate-400 mb-1 flex items-center gap-1.5">
                    <Network className="w-3.5 h-3.5 text-slate-400" />
                    Driver
                  </div>
                  <div className="text-xs font-mono font-semibold text-slate-200">{currentNetwork.Driver}</div>
                </div>

                <div className="p-3 rounded-lg bg-slate-900/60 border border-slate-800/80">
                  <div className="text-[11px] text-slate-400 mb-1 flex items-center gap-1.5">
                    <Globe className="w-3.5 h-3.5 text-slate-400" />
                    Scope
                  </div>
                  <div className="text-xs font-mono font-semibold text-slate-200">{currentNetwork.Scope}</div>
                </div>

                <div className="p-3 rounded-lg bg-slate-900/60 border border-slate-800/80">
                  <div className="text-[11px] text-slate-400 mb-1 flex items-center gap-1.5">
                    <Shield className="w-3.5 h-3.5 text-slate-400" />
                    Flags
                  </div>
                  <div className="flex flex-wrap gap-1">
                    {currentNetwork.Internal && (
                      <Badge variant="outline" className="text-[9px] px-1 py-0 bg-slate-800 border-slate-700 text-slate-300">
                        internal
                      </Badge>
                    )}
                    {currentNetwork.Attachable && (
                      <Badge variant="outline" className="text-[9px] px-1 py-0 bg-slate-800 border-slate-700 text-slate-300">
                        attachable
                      </Badge>
                    )}
                    {currentNetwork.EnableIPv6 && (
                      <Badge variant="outline" className="text-[9px] px-1 py-0 bg-slate-800 border-slate-700 text-slate-300">
                        ipv6
                      </Badge>
                    )}
                    {!currentNetwork.Internal && !currentNetwork.Attachable && !currentNetwork.EnableIPv6 && (
                      <span className="text-xs text-slate-500">-</span>
                    )}
                  </div>
                </div>
              </div>

              {/* IPAM Subnets */}
              <div>
                <h3 className="text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">
                  IPAM Configurations
                </h3>
                {ipamConfigs.length === 0 ? (
                  <p className="text-xs text-slate-500 italic">No IPAM subnets configured.</p>
                ) : (
                  <div className="space-y-2">
                    {ipamConfigs.map((cfg, idx) => (
                      <div
                        key={idx}
                        className="p-3 rounded-lg bg-slate-900/50 border border-slate-800 font-mono text-xs space-y-1"
                      >
                        {cfg.Subnet && (
                          <div className="flex justify-between">
                            <span className="text-slate-400 font-sans">Subnet:</span>
                            <span className="text-slate-200">{cfg.Subnet}</span>
                          </div>
                        )}
                        {cfg.Gateway && (
                          <div className="flex justify-between">
                            <span className="text-slate-400 font-sans">Gateway:</span>
                            <span className="text-slate-200">{cfg.Gateway}</span>
                          </div>
                        )}
                        {cfg.IPRange && (
                          <div className="flex justify-between">
                            <span className="text-slate-400 font-sans">IP Range:</span>
                            <span className="text-slate-200">{cfg.IPRange}</span>
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                )}
              </div>

              {/* Connected Containers */}
              <div>
                <div className="flex items-center justify-between mb-2">
                  <h3 className="text-xs font-semibold text-slate-300 uppercase tracking-wider">
                    Connected Containers
                  </h3>
                  <Badge variant="outline" className="text-[10px] font-mono bg-slate-900 border-slate-700 text-slate-400">
                    {containers.length}
                  </Badge>
                </div>
                {containers.length === 0 ? (
                  <p className="text-xs text-slate-500 italic">No containers attached to this network.</p>
                ) : (
                  <div className="border border-slate-800 rounded-lg overflow-hidden divide-y divide-slate-800/80">
                    {containers.map(([cid, endpoint]) => (
                      <div key={cid} className="p-3 bg-slate-900/30 flex items-center justify-between gap-3 text-xs">
                        <div className="flex items-center gap-2 min-w-0">
                          <Box className="w-3.5 h-3.5 text-slate-400 flex-shrink-0" />
                          <div className="min-w-0">
                            <div className="font-medium text-slate-200 truncate">{endpoint.Name || cid.substring(0, 12)}</div>
                            <div className="text-[10px] text-slate-400 font-mono truncate">
                              ID: {cid.substring(0, 12)}
                            </div>
                          </div>
                        </div>
                        <div className="text-right flex-shrink-0 font-mono text-[11px] text-slate-300">
                          {endpoint.IPv4Address || endpoint.IPv6Address || '-'}
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>

              {/* Labels */}
              {currentNetwork.Labels && Object.keys(currentNetwork.Labels).length > 0 && (
                <div>
                  <h3 className="text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">
                    Labels
                  </h3>
                  <div className="max-h-48 overflow-y-auto rounded-lg border border-slate-800 bg-slate-900/40 p-2.5 font-mono text-xs divide-y divide-slate-800/60">
                    {Object.entries(currentNetwork.Labels).map(([key, value]) => (
                      <div key={key} className="py-1 flex justify-between gap-2">
                        <span className="text-slate-400 truncate">{key}:</span>
                        <span className="text-slate-200 text-right truncate">{value}</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}
        </ScrollArea>
      </SheetContent>
    </Sheet>
  );
};
