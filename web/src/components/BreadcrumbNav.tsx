import React from 'react';
import { Link } from '@tanstack/react-router';

export interface BreadcrumbItem {
  label: React.ReactNode;
  to?: string;
  params?: Record<string, string>;
  icon?: React.ComponentType<{ className?: string }>;
}

export interface BreadcrumbNavProps {
  items: BreadcrumbItem[];
  className?: string;
}

export const BreadcrumbNav: React.FC<BreadcrumbNavProps> = ({ items, className = '' }) => {
  return (
    <nav aria-label="Breadcrumb" className={`flex items-center flex-wrap gap-2 text-xs font-mono ${className}`.trim()}>
      {items.map((item, index) => {
        const isLast = index === items.length - 1;
        const Icon = item.icon;

        return (
          <React.Fragment key={index}>
            {index > 0 && <span className="text-slate-600">/</span>}
            {isLast ? (
              <span className="text-white font-medium flex items-center gap-1.5">
                {Icon && <Icon className="w-3.5 h-3.5 text-slate-400" />}
                <span>{item.label}</span>
              </span>
            ) : item.to ? (
              <Link
                to={item.to}
                params={item.params}
                className="text-slate-400 hover:text-white transition-colors flex items-center gap-1.5"
              >
                {Icon && <Icon className="w-3.5 h-3.5 text-slate-400" />}
                <span>{item.label}</span>
              </Link>
            ) : (
              <span className="text-slate-400 flex items-center gap-1.5">
                {Icon && <Icon className="w-3.5 h-3.5 text-slate-400" />}
                <span>{item.label}</span>
              </span>
            )}
          </React.Fragment>
        );
      })}
    </nav>
  );
};
