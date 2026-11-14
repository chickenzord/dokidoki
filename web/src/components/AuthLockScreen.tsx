import React, { useState } from 'react';
import { api } from '../services/api';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from './ui/dialog';
import { Button } from './ui/button';
import { Input } from './ui/input';
import { Activity, Lock, AlertCircle, ArrowRight } from 'lucide-react';

interface AuthLockScreenProps {
  open: boolean;
  onSuccess: () => void;
}

export const AuthLockScreen: React.FC<AuthLockScreenProps> = ({ open, onSuccess }) => {
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!password) {
      setError('Please enter your password');
      return;
    }

    setLoading(true);
    setError(null);
    try {
      await api.login(password);
      setPassword('');
      onSuccess();
    } catch (err: any) {
      setError(err?.message || 'Invalid password');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={() => {}}>
      <DialogContent
        hideDefaultClose
        className="sm:max-w-md bg-slate-950 border-slate-800 text-slate-100 shadow-2xl p-6 sm:p-8"
      >
        <div className="flex flex-col items-center text-center space-y-3">
          <div className="w-12 h-12 rounded-xl bg-gradient-to-tr from-rose-600 via-pink-500 to-rose-400 flex items-center justify-center shadow-lg shadow-rose-950/50">
            <Activity className="w-6 h-6 text-white" />
          </div>
          <DialogHeader className="space-y-1">
            <DialogTitle className="text-xl font-bold tracking-tight text-white text-center">
              Welcome to Dokidoki
            </DialogTitle>
            <DialogDescription className="text-slate-400 text-sm text-center">
              Authentication required. Enter your admin password to access the cluster.
            </DialogDescription>
          </DialogHeader>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4 mt-4">
          {error && (
            <div className="flex items-center gap-2 p-3 text-sm rounded-lg bg-rose-500/10 border border-rose-500/30 text-rose-400">
              <AlertCircle className="w-4 h-4 shrink-0" />
              <span>{error}</span>
            </div>
          )}

          <div className="space-y-1.5">
            <label className="text-xs font-medium text-slate-300">Admin Password</label>
            <div className="relative">
              <Lock className="w-4 h-4 text-slate-500 absolute left-3 top-2.5" />
              <Input
                type="password"
                placeholder="Enter password..."
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                disabled={loading}
                autoFocus
                className="pl-9 bg-slate-900 border-slate-800 text-white placeholder:text-slate-500 focus:border-rose-500"
              />
            </div>
          </div>

          <Button
            type="submit"
            disabled={loading}
            className="w-full bg-rose-600 hover:bg-rose-500 text-white font-medium flex items-center justify-center gap-2 shadow-lg shadow-rose-950/30"
          >
            {loading ? (
              'Authenticating...'
            ) : (
              <>
                <span>Unlock Console</span>
                <ArrowRight className="w-4 h-4" />
              </>
            )}
          </Button>
        </form>
      </DialogContent>
    </Dialog>
  );
};
