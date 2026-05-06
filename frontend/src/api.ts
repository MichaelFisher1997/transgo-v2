import { FolderInfo, FoldersResponse, EncodeJob, EncodeResult, EncodeFileResult, EncodeJobEvent, Stats, SourceProbeSummary, AppSettings } from './types';

const API_BASE = '/api';

async function fetchJSON<T>(url: string, options?: RequestInit): Promise<T> {
  const controller = new AbortController();
  const timeout = window.setTimeout(() => controller.abort(), 30000);
  let res: Response;
  try {
    res = await fetch(url, { ...options, signal: controller.signal });
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') {
      throw new Error('Request timed out');
    }
    throw err;
  } finally {
    window.clearTimeout(timeout);
  }
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: 'Unknown error' }));
    throw new Error(err.error || `HTTP ${res.status}`);
  }
  return res.json();
}

export const api = {
  getFolders: () => fetchJSON<FoldersResponse>(`${API_BASE}/folders`),
  
  getFolderContents: (path: string) => fetchJSON<FolderInfo>(`${API_BASE}/folders/${encodeURIComponent(path)}`),

  deleteMediaFile: (path: string) =>
    fetchJSON<{ deleted: boolean; path: string; size: number }>(`${API_BASE}/files`, {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path }),
    }),

  probeSources: (paths: string[]) =>
    fetchJSON<SourceProbeSummary>(`${API_BASE}/probe`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ paths }),
    }),
  
  createJob: (sourcePath: string, preset: string = 'av1_qsv_balanced28') =>
    fetchJSON<{ id: number; status: string }>(`${API_BASE}/jobs`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ source_path: sourcePath, preset }),
    }),

  createJobsBatch: (sourcePaths: string[], preset: string = 'av1_qsv_balanced28') =>
    fetchJSON<{
      created: { id: number; source_path: string; status: string }[];
      skipped: { source_path: string; error: string }[];
      failed: { source_path: string; error: string }[];
    }>(`${API_BASE}/jobs/batch`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ source_paths: sourcePaths, preset }),
    }),
  
  getJobs: () => fetchJSON<EncodeJob[]>(`${API_BASE}/jobs`),
  
  getJob: (id: number) => fetchJSON<EncodeJob>(`${API_BASE}/jobs/${id}`),
  
  cancelJob: (id: number) =>
    fetchJSON<{ status: string }>(`${API_BASE}/jobs/${id}/cancel`, { method: 'POST' }),

  retryJob: (id: number) =>
    fetchJSON<{ id: number; status: string; retried_from: number }>(`${API_BASE}/jobs/${id}/retry`, { method: 'POST' }),

  deleteJob: (id: number) =>
    fetchJSON<{ status: string; temp_deleted: number; temp_failed: number }>(`${API_BASE}/jobs/${id}`, { method: 'DELETE' }),

  clearJobs: () =>
    fetchJSON<{ cleared: number; skipped: number; temp_deleted: number; temp_failed: number }>(`${API_BASE}/jobs`, { method: 'DELETE' }),
  
  cleanupJob: (id: number, dryRun = false) =>
    fetchJSON<{ deleted: number; failed: number; dry_run: boolean; files: string[] }>(`${API_BASE}/jobs/${id}/cleanup${dryRun ? '?dry_run=true' : ''}`, { method: 'DELETE' }),

  discardJob: (id: number, dryRun = false) =>
    fetchJSON<{ deleted: number; missing: number; failed: number; dry_run: boolean; files: string[] }>(`${API_BASE}/jobs/${id}/discard${dryRun ? '?dry_run=true' : ''}`, { method: 'DELETE' }),

  getJobEvents: (id: number) => fetchJSON<EncodeJobEvent[]>(`${API_BASE}/jobs/${id}/events`),
  
  getResults: () => fetchJSON<EncodeResult[]>(`${API_BASE}/results`),
  
  getResult: (id: number) =>
    fetchJSON<{ result: EncodeResult; files: EncodeFileResult[]; events: EncodeJobEvent[] }>(`${API_BASE}/results/${id}`),
  
  getStats: () => fetchJSON<Stats>(`${API_BASE}/stats`),

  getSettings: () => fetchJSON<AppSettings>(`${API_BASE}/settings`),

  updateSettings: (settings: AppSettings) =>
    fetchJSON<AppSettings>(`${API_BASE}/settings`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(settings),
    }),

  getPreflight: () => fetchJSON<Record<string, { ok: boolean; message: string }>>(`${API_BASE}/encoder/preflight`),
};
