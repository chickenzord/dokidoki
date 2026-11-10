import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function getHostForContainer(hostEndpoint?: string, hostAddress?: string): string {
  const target = hostAddress || hostEndpoint;
  if (!target) {
    return typeof window !== 'undefined' ? window.location.hostname : 'localhost';
  }
  try {
    const urlStr = target.startsWith('http://') || target.startsWith('https://')
      ? target
      : `http://${target}`;
    const parsed = new URL(urlStr);
    return parsed.hostname || (typeof window !== 'undefined' ? window.location.hostname : 'localhost');
  } catch {
    return typeof window !== 'undefined' ? window.location.hostname : 'localhost';
  }
}
