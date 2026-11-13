import React from 'react';
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from './ui/sheet';
import { Button } from './ui/button';
import { ScrollArea } from './ui/scroll-area';
import { useVolumeDetailQuery } from '../hooks/useClusterData';
import { HardDrive, Server, Globe, RefreshCw, Loader2, Folder, Calendar } from 'lucide-react';
import { ClusterVolume } from '../types';

interface VolumeDetailSheetProps {
  volume: ClusterVolume | null;
  isOpen: boolean;
  onClose: () => void;
}

export const VolumeDetailSheet: React.FC<VolumeDetailSheetProps> = ({
  volume,
  isOpen,
  onClose,
}) => {
  const { data: detailData, isLoading, refetch, isFetching } = useVolumeDetailQuery(
    volume?.hostEndpoint || '',
    volume?.Name || '',
    isOpen && Boolean(volume)
  );

  if (!volume) return null;

  const currentVolume = detailData || volume;

  return (
    <Sheet open={isOpen} onOpenChange={(open) => !open && onClose()}>
      <SheetContent className="w-full sm:max-w-2xl bg-slate-950 border-slate-800 text-slate-100 p-0 flex flex-col">
        <SheetHeader className="p-6 border-b border-slate-800 flex-shrink-0">
          <div className="flex items-center justify-between gap-3">
            <div className="flex items-center gap-2.5 min-w-0">
              <div className="p-2 rounded-lg bg-slate-900 border border-slate-800 text-slate-300">
                <HardDrive className="w-5 h-5 text-slate-400" />
              </div>
              <div className="min-w-0">
                <SheetTitle className="text-lg font-bold text-slate-100 truncate flex items-center gap-2">
                  <span>{currentVolume.Name}</span>
                </SheetTitle>
                <SheetDescription className="text-xs text-slate-400 font-mono truncate">
                  Driver: {currentVolume.Driver}
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
              {/* Properties Grid */}
              <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
                <div className="p-3 rounded-lg bg-slate-900/60 border border-slate-800/80">
                  <div className="text-[11px] text-slate-400 mb-1 flex items-center gap-1.5">
                    <Server className="w-3.5 h-3.5 text-slate-400" />
                    Host Node
                  </div>
                  <div className="text-xs font-semibold text-slate-200 truncate">{volume.hostName}</div>
                </div>

                <div className="p-3 rounded-lg bg-slate-900/60 border border-slate-800/80">
                  <div className="text-[11px] text-slate-400 mb-1 flex items-center gap-1.5">
                    <HardDrive className="w-3.5 h-3.5 text-slate-400" />
                    Driver
                  </div>
                  <div className="text-xs font-mono font-semibold text-slate-200">{currentVolume.Driver}</div>
                </div>

                <div className="p-3 rounded-lg bg-slate-900/60 border border-slate-800/80">
                  <div className="text-[11px] text-slate-400 mb-1 flex items-center gap-1.5">
                    <Globe className="w-3.5 h-3.5 text-slate-400" />
                    Scope
                  </div>
                  <div className="text-xs font-mono font-semibold text-slate-200">{currentVolume.Scope || 'local'}</div>
                </div>
              </div>

              {/* Mountpoint */}
              <div>
                <div className="text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2 flex items-center gap-1.5">
                  <Folder className="w-3.5 h-3.5 text-slate-400" />
                  Mountpoint on Host
                </div>
                <div className="p-3 rounded-lg bg-slate-900/60 border border-slate-800 font-mono text-xs text-slate-200 break-all select-all">
                  {currentVolume.Mountpoint || 'N/A'}
                </div>
              </div>

              {/* Created At */}
              {currentVolume.CreatedAt && (
                <div>
                  <div className="text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2 flex items-center gap-1.5">
                    <Calendar className="w-3.5 h-3.5 text-slate-400" />
                    Created At
                  </div>
                  <div className="p-3 rounded-lg bg-slate-900/60 border border-slate-800 font-mono text-xs text-slate-300">
                    {new Date(currentVolume.CreatedAt).toLocaleString()}
                  </div>
                </div>
              )}

              {/* Options */}
              {currentVolume.Options && Object.keys(currentVolume.Options).length > 0 && (
                <div>
                  <h3 className="text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">
                    Driver Options
                  </h3>
                  <div className="rounded-lg border border-slate-800 bg-slate-900/40 p-2.5 font-mono text-xs divide-y divide-slate-800/60">
                    {Object.entries(currentVolume.Options).map(([key, value]) => (
                      <div key={key} className="py-1 flex justify-between gap-2">
                        <span className="text-slate-400 truncate">{key}:</span>
                        <span className="text-slate-200 text-right truncate">{value}</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* Labels */}
              {currentVolume.Labels && Object.keys(currentVolume.Labels).length > 0 && (
                <div>
                  <h3 className="text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">
                    Labels
                  </h3>
                  <div className="max-h-48 overflow-y-auto rounded-lg border border-slate-800 bg-slate-900/40 p-2.5 font-mono text-xs divide-y divide-slate-800/60">
                    {Object.entries(currentVolume.Labels).map(([key, value]) => (
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
