import React, { useState, useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { ClusterStack, StackFile, CreateStackFile, CreateStackRequest, StackSummary } from '../types';
import { useCreateStackMutation } from '../hooks/useClusterData';
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
import { Tabs, TabsList, TabsTrigger, TabsContent } from './ui/tabs';
import { Server, FileCode, AlertCircle, CheckCircle2, Loader2, Files } from 'lucide-react';
import { CodeEditor } from './CodeEditor';

interface ImportStackDialogProps {
  isOpen: boolean;
  onClose: () => void;
  stack: ClusterStack | null;
  loadedFiles?: StackFile[];
  onImportSuccess?: (result: StackSummary) => void;
}

function formatBytes(bytes?: number): string {
  if (bytes === undefined || bytes === null || bytes === 0) return '0 B';
  if (bytes < 1024) return `${bytes} B`;
  return `${(bytes / 1024).toFixed(1)} KB`;
}

export const ImportStackDialog: React.FC<ImportStackDialogProps> = ({
  isOpen,
  onClose,
  stack,
  loadedFiles,
  onImportSuccess,
}) => {
  const queryClient = useQueryClient();
  const [stackName, setStackName] = useState('');
  const [files, setFiles] = useState<CreateStackFile[]>([]);
  const [activeFileName, setActiveFileName] = useState<string>('');
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  const createStackMutation = useCreateStackMutation();

  useEffect(() => {
    if (stack && isOpen) {
      setStackName(stack.name);
      setError(null);
      setSuccess(false);

      if (loadedFiles && loadedFiles.length > 0) {
        const initialFiles: CreateStackFile[] = loadedFiles.map((f) => ({
          name: f.name,
          content: f.content || '',
        }));
        setFiles(initialFiles);
        const composeFile = loadedFiles.find((f) => f.isCompose);
        setActiveFileName(composeFile?.name || loadedFiles[0]?.name || '');
      } else {
        const defaultContent =
          `# External Compose Stack: ${stack.name}\n` +
          `# Host: ${stack.hostName}\n` +
          (stack.composePath ? `# Compose Path on host: ${stack.composePath}\n` : '') +
          `# Provide or verify the compose configuration below:\n\n` +
          `services:\n` +
          stack.services
            .map((svc) => `  ${svc}:\n    # Configuration for ${svc}\n`)
            .join('');
        setFiles([{ name: 'compose.yaml', content: defaultContent }]);
        setActiveFileName('compose.yaml');
      }
    }
  }, [stack, loadedFiles, isOpen]);

  const handleContentChange = (name: string, newContent: string) => {
    setFiles((prev) =>
      prev.map((f) => (f.name === name ? { ...f, content: newContent } : f))
    );
  };

  const handleImport = async () => {
    if (!stack) return;
    const trimmedName = stackName.trim();
    if (!trimmedName) {
      setError('Stack name is required');
      return;
    }
    setError(null);

    const requestData: CreateStackRequest = {
      name: trimmedName,
      files: files.map((f) => ({
        name: f.name,
        content: f.content,
      })),
    };

    try {
      const created = await createStackMutation.mutateAsync({
        data: requestData,
        hostEndpoint: stack.hostEndpoint,
      });

      setSuccess(true);
      queryClient.invalidateQueries({ queryKey: ['cluster-stacks'] });
      queryClient.invalidateQueries({ queryKey: ['cluster-containers'] });
      queryClient.invalidateQueries({ queryKey: ['stack-files'] });
      queryClient.invalidateQueries({ queryKey: ['stack-detail'] });
      onImportSuccess?.(created);

      setTimeout(() => {
        onClose();
        setSuccess(false);
      }, 800);
    } catch (err: any) {
      setError(err.message || 'Failed to import stack');
    }
  };

  if (!stack) return null;

  return (
    <Dialog open={isOpen} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto bg-slate-900 border-slate-800 text-slate-100 sm:rounded-xl">
        <DialogHeader>
          <div className="flex items-center gap-2">
            <DialogTitle className="text-lg font-bold">Import External Stack</DialogTitle>
            <Badge variant="outline" className="text-xs bg-slate-800/80 text-slate-300 border-slate-700 flex items-center gap-1 font-mono">
              <Server className="w-3 h-3 text-slate-400" />
              {stack.hostName}
            </Badge>
          </div>
          <DialogDescription className="text-xs text-slate-400">
            Import this external compose stack into Dokidoki management on host <strong className="text-slate-300">{stack.hostName}</strong>. Dokidoki will save the configuration files to its managed stack directory.
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
              <span>Stack imported successfully!</span>
            </div>
          )}

          <div className="space-y-1.5">
            <label className="text-xs font-medium text-slate-300">Stack Name</label>
            <Input
              value={stackName}
              onChange={(e) => setStackName(e.target.value)}
              placeholder="e.g. my-app"
              className="bg-slate-950 border-slate-800 text-slate-100 font-mono text-xs focus:ring-rose-500"
            />
          </div>

          {/* Files Summary Header */}
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <label className="text-xs font-medium text-slate-300 flex items-center gap-1.5">
                <Files className="w-3.5 h-3.5 text-slate-400" />
                Files to Import ({files.length})
              </label>
            </div>

            {/* Files preview badges */}
            <div className="flex flex-wrap gap-1.5 p-2.5 rounded-lg bg-slate-950/60 border border-slate-800">
              {files.map((file) => {
                const meta = loadedFiles?.find((f) => f.name === file.name);
                const isSelected = activeFileName === file.name;
                return (
                  <button
                    key={file.name}
                    type="button"
                    onClick={() => setActiveFileName(file.name)}
                    className={`flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-mono transition-colors ${
                      isSelected
                        ? 'bg-slate-800 text-white ring-1 ring-rose-500/50'
                        : 'bg-slate-900/80 text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
                    }`}
                  >
                    <FileCode className="w-3 h-3 text-slate-400" />
                    <span>{file.name}</span>
                    <span className="text-[10px] text-slate-500">
                      ({formatBytes(meta?.size ?? file.content.length)})
                    </span>
                    {meta?.isCompose && (
                      <span className="w-1.5 h-1.5 rounded-full bg-rose-400" />
                    )}
                  </button>
                );
              })}
            </div>

            {/* Files Tabs Editor/Viewer */}
            {files.length > 0 && (
              <Tabs
                value={activeFileName}
                onValueChange={setActiveFileName}
                className="w-full mt-2"
              >
                <TabsList className="bg-slate-950/80 border border-slate-800/80 p-0.5 h-auto flex flex-wrap gap-1">
                  {files.map((file) => (
                    <TabsTrigger
                      key={file.name}
                      value={file.name}
                      className="text-xs font-mono px-3 py-1.5 data-[state=active]:bg-slate-800 data-[state=active]:text-white text-slate-400"
                    >
                      {file.name}
                    </TabsTrigger>
                  ))}
                </TabsList>

                {files.map((file) => (
                  <TabsContent key={file.name} value={file.name} className="mt-2 outline-none">
                    <CodeEditor
                      value={file.content}
                      onChange={(newContent) => handleContentChange(file.name, newContent)}
                      filename={file.name}
                      readOnly={false}
                      minHeight="220px"
                      maxHeight="420px"
                      placeholder={`Content for ${file.name}`}
                    />
                  </TabsContent>
                ))}
              </Tabs>
            )}
          </div>
        </div>

        <DialogFooter className="gap-2 sm:gap-0 pt-2">
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
            onClick={handleImport}
            disabled={createStackMutation.isPending || !stackName.trim() || files.length === 0 || success}
            className="bg-rose-600 hover:bg-rose-500 text-white text-xs font-medium"
          >
            {createStackMutation.isPending ? (
              <>
                <Loader2 className="w-3.5 h-3.5 mr-1.5 animate-spin" />
                Importing...
              </>
            ) : success ? (
              'Imported!'
            ) : (
              'Import as Managed Stack'
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
