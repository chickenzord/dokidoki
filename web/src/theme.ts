/**
 * Centralized theme tokens for Dokidoki resources and UI elements.
 * Modify these to adjust color coding or revert to colored accents easily.
 */

// If set to true, resource types use subtle neutral slate tones, reserving colors strictly for status.
// Set to false to restore colorful resource accents.
export const NEUTRAL_RESOURCE_THEME = true;

export const resourceTheme = {
  stack: {
    icon: NEUTRAL_RESOURCE_THEME ? 'text-slate-400' : 'text-rose-400',
    activeBg: NEUTRAL_RESOURCE_THEME ? 'bg-slate-800 text-white font-medium' : 'bg-rose-500/15 text-rose-300 font-medium',
    border: NEUTRAL_RESOURCE_THEME ? 'border-slate-700/60' : 'border-rose-500/30',
  },
  container: {
    icon: NEUTRAL_RESOURCE_THEME ? 'text-slate-400' : 'text-blue-400',
    activeBg: NEUTRAL_RESOURCE_THEME ? 'bg-slate-800 text-white font-medium' : 'bg-blue-500/15 text-blue-300 font-medium',
    border: NEUTRAL_RESOURCE_THEME ? 'border-slate-700/60' : 'border-blue-500/30',
  },
  node: {
    icon: NEUTRAL_RESOURCE_THEME ? 'text-slate-400' : 'text-emerald-400',
    activeBg: NEUTRAL_RESOURCE_THEME ? 'bg-slate-800 text-white font-medium' : 'bg-emerald-500/15 text-emerald-300 font-medium',
    border: NEUTRAL_RESOURCE_THEME ? 'border-slate-700/60' : 'border-emerald-500/30',
  },
  network: {
    icon: NEUTRAL_RESOURCE_THEME ? 'text-slate-400' : 'text-indigo-400',
    activeBg: NEUTRAL_RESOURCE_THEME ? 'bg-slate-800 text-white font-medium' : 'bg-indigo-500/15 text-indigo-300 font-medium',
    border: NEUTRAL_RESOURCE_THEME ? 'border-slate-700/60' : 'border-indigo-500/30',
  },
  volume: {
    icon: NEUTRAL_RESOURCE_THEME ? 'text-slate-400' : 'text-amber-400',
    activeBg: NEUTRAL_RESOURCE_THEME ? 'bg-slate-800 text-white font-medium' : 'bg-amber-500/15 text-amber-300 font-medium',
    border: NEUTRAL_RESOURCE_THEME ? 'border-slate-700/60' : 'border-amber-500/30',
  },
};
