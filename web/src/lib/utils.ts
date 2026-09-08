import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function getHostForContainer(hostEndpoint?: string): string {
  if (!hostEndpoint) {
    return typeof window !== 'undefined' ? window.location.hostname : 'localhost';
  }
  try {
    const urlStr = hostEndpoint.startsWith('http://') || hostEndpoint.startsWith('https://')
      ? hostEndpoint
      : `http://${hostEndpoint}`;
    const parsed = new URL(urlStr);
    return parsed.hostname || (typeof window !== 'undefined' ? window.location.hostname : 'localhost');
  } catch {
    return typeof window !== 'undefined' ? window.location.hostname : 'localhost';
  }
}
