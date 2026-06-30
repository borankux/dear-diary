// API base URL — when served from Go backend, it's same origin
const API_BASE = '';

let authToken: string | null = localStorage.getItem('diary_auth_token');

export function setAuthToken(token: string) {
  authToken = token;
  localStorage.setItem('diary_auth_token', token);
}

export function clearAuthToken() {
  authToken = null;
  localStorage.removeItem('diary_auth_token');
}

export function isAuthenticated(): boolean {
  return !!authToken;
}

async function apiGet<T>(path: string): Promise<T> {
  const headers: Record<string, string> = {};
  if (authToken) headers['Authorization'] = `Bearer ${authToken}`;
  const res = await fetch(`${API_BASE}/api${path}`, { headers });
  if (res.status === 401) {
    window.dispatchEvent(new Event('auth:required'));
  }
  if (!res.ok) throw new Error(`API ${path}: ${res.status} ${res.statusText}`);
  return res.json();
}

async function apiPost<T>(path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (authToken) headers['Authorization'] = `Bearer ${authToken}`;
  const res = await fetch(`${API_BASE}/api${path}`, {
    method: 'POST',
    headers,
    body: body ? JSON.stringify(body) : undefined,
  });
  if (res.status === 401) {
    window.dispatchEvent(new Event('auth:required'));
  }
  if (!res.ok) throw new Error(`API POST ${path}: ${res.status} ${res.statusText}`);
  return res.json();
}

export interface Stats {
  today: { date: string; exists: boolean; sections: number };
  candidateCount: number;
  todoCounts: { active: number; in_progress: number; done: number; wont_do: number; archived: number; other: number };
  memoryCount: number;
  diaryCount: number;
}

export interface Todo {
  id: number;
  text: string;
  status: string;
  priority?: number;
  hasPriority: boolean;
  sourceDate: string;
  sourceFile: string;
  evidenceText: string;
  createdAt: string;
  updatedAt: string;
}

export interface Candidate {
  id: number;
  type: string;
  title: string;
  content: string;
  status: string;
  sourceDate: string;
  sourceFile: string;
  evidenceText: string;
  confidence: number;
}

export interface MergeResult {
  candidateGroups: number;
  candidateMerged: number;
  todoGroups: number;
  todoMerged: number;
  memoryGroups: number;
  memoryMerged: number;
}

export interface BulkPromotionResult {
  ok: boolean;
  promotedTodos: number;
  promotedMemories: number;
  merge: MergeResult;
}

export interface MergeDuplicatesResult {
  ok: boolean;
  merged: MergeResult;
}

export interface Memory {
  id: number;
  topic: string;
  summary: string;
  sourceDate: string;
  sourceFile: string;
  createdAt: string;
}

export interface DiaryEntry {
  date: string;
  title: string;
  excerpt: string;
  html: string;
  month: string;
}

export interface CalendarMonth {
  month: string;
  title: string;
  count: number;
  daysInMonth: number;
  days: CalendarDay[];
}

export interface CalendarDay {
  day: number;
  date: string;
  isPadding: boolean;
  isWritten: boolean;
  isToday: boolean;
}

export interface SearchResult {
  date: string;
  title: string;
  lines: { lineNum: number; text: string }[];
}

export function getStats() { return apiGet<Stats>('/stats'); }
export function getTodos() { return apiGet<Todo[]>('/todos'); }
export function getCandidates() { return apiGet<Candidate[]>('/candidates'); }
export function getMemories() { return apiGet<Memory[]>('/memories'); }
export function getDiaries() { return apiGet<DiaryEntry[]>('/diaries'); }
export function getCalendar() { return apiGet<CalendarMonth[]>('/calendar'); }
export function getDiary(date: string) { return apiGet<DiaryEntry>(`/diaries/${date}`); }
export function searchDiaries(q: string) { return apiGet<SearchResult[]>(`/search?q=${encodeURIComponent(q)}`); }

export function updateTodoPriority(id: number, priority: number | null) {
  return apiPost<{ ok: boolean }>(`/todos/${id}/priority`, { priority });
}

export function updateTodoStatus(id: number, status: string) {
  return apiPost<{ ok: boolean }>(`/todos/${id}/status`, { status });
}

export function acceptCandidate(id: number) {
  return apiPost<{ ok: boolean }>(`/candidates/${id}/accept`);
}

export function rejectCandidate(id: number) {
  return apiPost<{ ok: boolean }>(`/candidates/${id}/reject`);
}

export function mergeDuplicateCandidates() {
  return apiPost<MergeDuplicatesResult>('/candidates/merge-duplicates');
}

export function promoteAllCandidates() {
  return apiPost<BulkPromotionResult>('/candidates/promote-all');
}

export async function login(password: string): Promise<{ token: string }> {
  const res = await fetch('/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ password }),
  });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}
