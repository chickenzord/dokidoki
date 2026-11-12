import React from 'react';

export interface FilterOption<T extends string> {
  value: T;
  label: string;
  dotColor?: string;
}

export interface FilterPillsProps<T extends string> {
  options: FilterOption<T>[];
  value: T;
  onChange: (val: T) => void;
  size?: 'sm' | 'xs';
}

export function FilterPills<T extends string>({
  options,
  value,
  onChange,
  size = 'sm',
}: FilterPillsProps<T>): React.ReactElement {
  const paddingClass = size === 'xs' ? 'px-2 py-0.5 text-[11px]' : 'px-2.5 py-1 text-xs';

  return (
    <div className="flex items-center rounded-lg bg-slate-900 border border-slate-800 p-0.5 text-xs">
      {options.map((opt) => {
        const isActive = opt.value === value;
        return (
          <button
            key={opt.value}
            type="button"
            onClick={() => onChange(opt.value)}
            className={`rounded-md transition-all flex items-center gap-1.5 ${paddingClass} ${
              isActive
                ? 'bg-slate-800 text-white font-medium shadow-sm'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            {opt.dotColor && (
              <span className={`w-1.5 h-1.5 rounded-full ${opt.dotColor}`} />
            )}
            <span>{opt.label}</span>
          </button>
        );
      })}
    </div>
  );
}
