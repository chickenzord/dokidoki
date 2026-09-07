import React, { useState, useEffect } from 'react';
import { ClusterContainer } from '../types';
import { useContainerComposeQuery, useCreateStackMutation } from '../hooks/useClusterData';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from './ui/dialog';
import { Button } from './ui/button';
import { Input } from './ui/input';
import { Badge } from './ui/badge';
import { Server, FileCode, AlertCircle, CheckCircle2, Loader2, Sparkles } from 'lucide-react';

interface GenerateComposeDialogProps {
  isOpen: boolean;
  onClose: () => void;
  container: ClusterContainer | null;
}

export const GenerateComposeDialog: React.FC<GenerateComposeDialogProps> = ({
  isOpen,
  onClose,
  container,
}) => {
  const [stackName, setStackName] = useState('');
  const [composeContent, setComposeContent] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  const hostEndpoint = container?.hostEndpoint || '';
  const containerId = container?.id || '';

  const { data: composeData, isLoading: isLoadingCompose } = useContainerComposeQuery(
    hostEndpoint,
    containerId
  );

  const createStackMutation = useCreateStackMutation();

  useEffect(() => {
    if (container) {
      const cleanName = container.name.replace(/^\//, '');
      setStackName(cleanName);
      setError(null);
      setSuccess(false);

      if (composeData?.content) {
        setComposeContent(composeData.content);
      } else {
        // Basic initial template while loading or fallback
        setComposeContent(
          `# Generated Compose Specification for ${cleanName}\n` +
          `services:\n` +
          `  ${cleanName}:\n` +
          `    image: ${container.image}\n` +
          `    container_name: ${cleanName}\n` +
          `    restart: unless-stopped\n`
        );
      }
    }
  }, [container, composeData]);

  const handleCreate = async () => {
    if (!container) return;
    setError(null);

    try {
      await createStackMutation.mutateAsync({
        data: {
          name: stackName.trim(),
          content: composeContent,
        },
        hostEndpoint: container.hostEndpoint,
      });

      setSuccess(true);
      setTimeout(() => {
        onClose();
        setSuccess(false);
      }, 800);
    } catch (err: any) {
      setError(err.message || 'Failed to create stack');
    }
  };

  if (!container) return null;

  return (
    <Dialog open={isOpen} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-2xl bg-slate-900 border-slate-800 text-slate-100 sm:rounded-xl">
        <DialogHeader>
          <div className="flex items-center gap-2">
            <DialogTitle className="text-lg font-bold flex items-center gap-2">
              <Sparkles className="w-4 h-4 text-amber-400" />
              Manage as Compose Stack
            </DialogTitle>
            <Badge variant="outline" className="text-xs bg-slate-800/80 text-slate-300 border-slate-700 flex items-center gap-1 font-mono">
              <Server className="w-3 h-3 text-slate-400" />
              {container.hostName}
            </Badge>
          </div>
          <DialogDescription className="text-xs text-slate-400">
            Convert standalone container <code className="text-slate-300 font-mono">{container.name.replace(/^\//, '')}</code> into a managed Docker Compose stack on <strong className="text-slate-300">{container.hostName}</strong>.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          {error && (
            <div className="p-3 bg-rose-950/40 border border-rose-800/60 rounded-lg text-rose-300 text-xs flex items-center gap-2">
              <AlertCircle className="w-4 h-4 text-rose-400 flex-shrink-0" />
              <span>{error}</span>
            </div>
          )}

          {success && (
            <div className="p-3 bg-emerald-950/40 border border-emerald-800/60 rounded-lg text-emerald-300 text-xs flex items-center gap-2">
              <CheckCircle2 className="w-4 h-4 text-emerald-400 flex-shrink-0" />
              <span>Compose stack created successfully!</span>
            </div>
          )}

          <div className="space-y-1.5">
            <label className="text-xs font-medium text-slate-300">Stack Name</label>
            <Input
              value={stackName}
              onChange={(e) => setStackName(e.target.value)}
              placeholder="e.g. redis-service"
              className="bg-slate-950 border-slate-800 text-slate-100 font-mono text-xs focus:ring-rose-500"
            />
          </div>

          <div className="space-y-1.5">
            <div className="flex items-center justify-between">
              <label className="text-xs font-medium text-slate-300 flex items-center gap-1.5">
                <FileCode className="w-3.5 h-3.5 text-slate-400" />
                Generated Compose YAML
              </label>
              {isLoadingCompose && (
                <span className="text-[11px] text-slate-500 flex items-center gap-1">
                  <Loader2 className="w-3 h-3 animate-spin" /> Generating compose specification...
                </span>
              )}
            </div>
            <textarea
              rows={12}
              value={composeContent}
              onChange={(e) => setComposeContent(e.target.value)}
              className="w-full bg-slate-950 border border-slate-800 rounded-md p-3 text-slate-200 font-mono text-xs leading-relaxed focus:outline-none focus:ring-1 focus:ring-rose-500/50 resize-y"
              placeholder="services:&#10;  ..."
              spellCheck={false}
            />
          </div>
        </div>

        <DialogFooter className="gap-2 sm:gap-0">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={onClose}
            className="text-slate-400 hover:text-slate-100 hover:bg-slate-800 text-xs"
          >
            Cancel
          </Button>
          <Button
            type="button"
            size="sm"
            onClick={handleCreate}
            disabled={createStackMutation.isPending || !stackName.trim() || success}
            className="bg-rose-600 hover:bg-rose-500 text-white text-xs font-medium"
          >
            {createStackMutation.isPending ? (
              <>
                <Loader2 className="w-3.5 h-3.5 mr-1.5 animate-spin" />
                Creating Stack...
              </>
            ) : success ? (
              'Created!'
            ) : (
              'Create Compose Stack'
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
