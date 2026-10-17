import { keepPreviousData, queryOptions, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./client.ts";
import type {
  AgesResponse,
  Breakdown,
  ChildrenResponse,
  DiffResponse,
  FindingsResponse,
  HistoryResponse,
  OpsResponse,
  Plan,
  PlanRequest,
  PlanResult,
  PresetView,
  ScanRequest,
  ScanState,
  SearchResponse,
  Settings,
  SliceResponse,
  Status,
  WebAccess,
} from "./types.gen.ts";

const q = encodeURIComponent;

export const useStatus = () => useQuery({ queryKey: ["status"], queryFn: () => api.get<Status>("/status") });
export const useVolume = () => useQuery({ queryKey: ["volume"], queryFn: () => api.get<Breakdown>("/volume"), retry: false });

export const useFindings = (ignored = false) =>
  useQuery({ queryKey: ["findings", ignored], queryFn: () => api.get<FindingsResponse>(`/findings${ignored ? "?ignored=1" : ""}`), retry: false });

export const sliceQuery = (path: string | undefined, depth: number, maxNodes: number) =>
  queryOptions({
    queryKey: ["tree", path ?? "", depth, maxNodes],
    queryFn: () => api.get<SliceResponse>(`/tree?depth=${depth}&maxNodes=${maxNodes}&minShare=0.003${path ? `&path=${q(path)}` : ""}`),
    staleTime: 30_000,
    retry: false,
  });

export const useSlice = (path: string | undefined, depth: number, maxNodes: number) =>
  useQuery({ ...sliceQuery(path, depth, maxNodes), placeholderData: keepPreviousData });

export const useAges = (path?: string) =>
  useQuery({
    queryKey: ["ages", path ?? ""],
    queryFn: () => api.get<AgesResponse>(`/ages${path ? `?path=${q(path)}` : ""}`),
    placeholderData: keepPreviousData,
    retry: false,
  });

export const useChildren = (path: string | undefined, small = false) =>
  useQuery({
    queryKey: ["children", path ?? "", small],
    queryFn: () => api.get<ChildrenResponse>(`/children?limit=2000${small ? "&small=1" : ""}${path ? `&path=${q(path)}` : ""}`),
    placeholderData: keepPreviousData,
    retry: false,
  });

export const useSearch = (term: string) =>
  useQuery({ queryKey: ["search", term], queryFn: () => api.get<SearchResponse>(`/search?limit=30&q=${q(term)}`), enabled: term.length > 1 });

export const usePresets = () => useQuery({ queryKey: ["presets"], queryFn: () => api.get<PresetView[]>("/presets") });
export const useOps = () => useQuery({ queryKey: ["ops"], queryFn: () => api.get<OpsResponse>("/ops?limit=500") });
export const useSettings = () => useQuery({ queryKey: ["settings"], queryFn: () => api.get<Settings>("/settings") });
export const useWebAccess = () => useQuery({ queryKey: ["web"], queryFn: () => api.get<WebAccess>("/web") });
export const useHistory = () => useQuery({ queryKey: ["history"], queryFn: () => api.get<HistoryResponse>("/history") });
export const useDiff = (from: string, to: string) =>
  useQuery({ queryKey: ["diff", from, to], queryFn: () => api.get<DiffResponse>(`/history/diff?from=${q(from)}&to=${q(to)}`), enabled: !!from && !!to, retry: false });

/** Everything that shows scan results; refreshed after a scan or a cleanup. */
export const resultKeys = [["status"], ["volume"], ["findings"], ["tree"], ["ages"], ["children"], ["search"], ["presets"], ["history"]];

export function useRefreshResults() {
  const qc = useQueryClient();
  return () => resultKeys.forEach((k) => qc.invalidateQueries({ queryKey: k }));
}

export function useStartScan() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: ScanRequest) => api.post<ScanState>("/scans", req),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["status"] }),
  });
}

export function useStopScan() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.del<ScanState>("/scans/current"),
    onSettled: () => qc.invalidateQueries({ queryKey: ["status"] }),
  });
}

export const useMakePlan = () => useMutation({ mutationFn: (req: PlanRequest) => api.post<Plan>("/plans", req) });

export function useRunPlan() {
  const refresh = useRefreshResults();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      await api.post<Plan>(`/plans/${id}/approve`);
      return api.post<PlanResult>(`/plans/${id}/execute`);
    },
    onSettled: () => {
      refresh();
      qc.invalidateQueries({ queryKey: ["ops"] });
    },
  });
}

export function useUndo() {
  const refresh = useRefreshResults();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.post(`/ops/${id}/undo`),
    onSettled: () => {
      refresh();
      qc.invalidateQueries({ queryKey: ["ops"] });
    },
  });
}

export function useSetNote() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (n: { key: string; text: string }) => api.post("/notes", n),
    onSuccess: () => ["findings", "children", "notes"].forEach((k) => qc.invalidateQueries({ queryKey: [k] })),
  });
}

export function useIgnore() {
  const refresh = useRefreshResults();
  return useMutation({
    mutationFn: ({ id, undo }: { id: string; undo?: boolean }) => (undo ? api.del(`/ignores/${q(id)}`) : api.post("/ignores", { findingId: id })),
    onSuccess: refresh,
  });
}

export function useSaveSettings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (s: Partial<Settings>) => api.post<Settings>("/settings", s),
    onSuccess: (s) => qc.setQueryData(["settings"], s),
  });
}

export const useReveal = () => useMutation({ mutationFn: (path: string) => api.post("/reveal", { path }) });

export const useNotes = () => useQuery({ queryKey: ["notes"], queryFn: () => api.get<Record<string, string>>("/notes") });
