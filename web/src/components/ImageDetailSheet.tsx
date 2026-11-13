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
import { useImageDetailQuery } from '../hooks/useClusterData';
import { Layers, Server, Calendar, RefreshCw, Loader2, HardDrive, Box, Tag } from 'lucide-react';
import { ClusterImage } from '../types';

interface ImageDetailSheetProps {
  image: ClusterImage | null;
  isOpen: boolean;
  onClose: () => void;
}

function formatBytes(bytes?: number): string {
  if (!bytes || bytes <= 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`;
}

export const ImageDetailSheet: React.FC<ImageDetailSheetProps> = ({
  image,
  isOpen,
  onClose,
}) => {
  const imgId = image?.Id || image?.ID || '';
  const { data: detailData, isLoading, refetch, isFetching } = useImageDetailQuery(
    image?.hostEndpoint || '',
    imgId,
    isOpen && Boolean(imgId)
  );

  if (!image) return null;

  const currentImage = detailData || image;
  const tags = (image.RepoTags && image.RepoTags.length > 0 ? image.RepoTags : ['<none>:<none>']);
  const digests = image.RepoDigests || [];
  const labels = currentImage.Labels || (currentImage.Config?.Labels) || {};

  return (
    <Sheet open={isOpen} onOpenChange={(open) => !open && onClose()}>
      <SheetContent className="w-full sm:max-w-2xl bg-slate-950 border-slate-800 text-slate-100 p-0 flex flex-col">
        <SheetHeader className="p-6 border-b border-slate-800 flex-shrink-0">
          <div className="flex items-center justify-between gap-3">
            <div className="flex items-center gap-2.5 min-w-0">
              <div className="p-2 rounded-lg bg-slate-900 border border-slate-800 text-slate-300">
                <Layers className="w-5 h-5 text-slate-400" />
              </div>
              <div className="min-w-0">
                <SheetTitle className="text-lg font-bold text-slate-100 truncate flex items-center gap-2">
                  <span>{tags[0]}</span>
                </SheetTitle>
                <SheetDescription className="text-xs text-slate-400 font-mono truncate">
                  ID: {imgId.replace('sha256:', '').substring(0, 16)}
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
              <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
                <div className="p-3 rounded-lg bg-slate-900/60 border border-slate-800/80">
                  <div className="text-[11px] text-slate-400 mb-1 flex items-center gap-1.5">
                    <Server className="w-3.5 h-3.5 text-slate-400" />
                    Host Node
                  </div>
                  <div className="text-xs font-semibold text-slate-200 truncate">{image.hostName}</div>
                </div>

                <div className="p-3 rounded-lg bg-slate-900/60 border border-slate-800/80">
                  <div className="text-[11px] text-slate-400 mb-1 flex items-center gap-1.5">
                    <HardDrive className="w-3.5 h-3.5 text-slate-400" />
                    Size
                  </div>
                  <div className="text-xs font-mono font-semibold text-slate-200">{formatBytes(image.Size)}</div>
                </div>

                <div className="p-3 rounded-lg bg-slate-900/60 border border-slate-800/80">
                  <div className="text-[11px] text-slate-400 mb-1 flex items-center gap-1.5">
                    <Box className="w-3.5 h-3.5 text-slate-400" />
                    Containers
                  </div>
                  <div className="text-xs font-mono font-semibold text-slate-200">{image.Containers}</div>
                </div>

                <div className="p-3 rounded-lg bg-slate-900/60 border border-slate-800/80">
                  <div className="text-[11px] text-slate-400 mb-1 flex items-center gap-1.5">
                    <Calendar className="w-3.5 h-3.5 text-slate-400" />
                    Created
                  </div>
                  <div className="text-xs font-mono text-slate-300 truncate">
                    {image.Created ? new Date(image.Created * 1000).toLocaleDateString() : '-'}
                  </div>
                </div>
              </div>

              {/* Repo Tags */}
              <div>
                <div className="text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2 flex items-center gap-1.5">
                  <Tag className="w-3.5 h-3.5 text-slate-400" />
                  Repo Tags
                </div>
                <div className="space-y-1.5">
                  {tags.map((tag, idx) => (
                    <div
                      key={idx}
                      className="p-2.5 rounded-lg bg-slate-900/50 border border-slate-800 font-mono text-xs text-slate-200 break-all"
                    >
                      {tag}
                    </div>
                  ))}
                </div>
              </div>

              {/* Repo Digests */}
              {digests.length > 0 && (
                <div>
                  <h3 className="text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">
                    Repo Digests
                  </h3>
                  <div className="space-y-1.5">
                    {digests.map((dig, idx) => (
                      <div
                        key={idx}
                        className="p-2.5 rounded-lg bg-slate-900/50 border border-slate-800 font-mono text-[11px] text-slate-400 break-all"
                      >
                        {dig}
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* Full Image ID */}
              <div>
                <h3 className="text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">
                  Full Image ID
                </h3>
                <div className="p-2.5 rounded-lg bg-slate-900/50 border border-slate-800 font-mono text-xs text-slate-400 break-all select-all">
                  {imgId}
                </div>
              </div>

              {/* Labels */}
              {Object.keys(labels).length > 0 && (
                <div>
                  <h3 className="text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">
                    Labels
                  </h3>
                  <div className="max-h-48 overflow-y-auto rounded-lg border border-slate-800 bg-slate-900/40 p-2.5 font-mono text-xs divide-y divide-slate-800/60">
                    {Object.entries(labels).map(([key, value]) => (
                      <div key={key} className="py-1 flex justify-between gap-2">
                        <span className="text-slate-400 truncate">{key}:</span>
                        <span className="text-slate-200 text-right truncate">{String(value)}</span>
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
