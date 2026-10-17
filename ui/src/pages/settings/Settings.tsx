import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { api, ApiError } from "#/api/client.ts";
import { useFindings, useIgnore, useSaveSettings, useSettings, useStatus, useWebAccess } from "#/api/hooks.ts";
import type { Settings as S } from "#/api/types.gen.ts";
import { Loading, Problem } from "#/ui/states.tsx";

export default function Settings() {
  return (
    <div className="page-narrow settings">
      <h1>Settings</h1>
      <Scan />
      <Protect />
      <Ignored />
      <Access />
      <Web />
      <Theme />
    </div>
  );
}

function Scan() {
  const settings = useSettings();
  const save = useSaveSettings();
  const [draft, setDraft] = useState<S | null>(null);
  useEffect(() => setDraft(settings.data ?? null), [settings.data]);
  if (settings.error) return <Problem error={settings.error} />;
  if (!draft) return <Loading />;
  const set = <K extends keyof S>(k: K, v: S[K]) => setDraft({ ...draft, [k]: v });
  return (
    <section className="panel setting">
      <h2>Scanning</h2>
      <label>
        Folder to scan <span className="faint">(empty: the whole disk)</span>
        <input className="input" value={draft.root} placeholder="/System/Volumes/Data" onChange={(e) => set("root", e.target.value)} />
      </label>
      <div className="row" style={{ flexWrap: "wrap", gap: 16 }}>
        <label>
          Parallel readers <span className="faint">(0: automatic)</span>
          <input className="input" type="number" min={0} max={64} value={draft.threads} onChange={(e) => set("threads", Number(e.target.value))} />
        </label>
        <label>
          Untouched after (days)
          <input className="input" type="number" min={1} max={3650} value={draft.staleDays} onChange={(e) => set("staleDays", Number(e.target.value))} />
        </label>
        <label>
          Large file from
          <input className="input" value={draft.largeFile} onChange={(e) => set("largeFile", e.target.value)} />
        </label>
      </div>
      <div className="row">
        <button className="btn primary" disabled={save.isPending} onClick={() => save.mutate(draft)}>Save</button>
        {save.error && <span className="error">{(save.error as Error).message}</span>}
        {save.isSuccess && <span className="muted">Saved. Changes apply to the next scan.</span>}
      </div>
    </section>
  );
}

function Protect() {
  const settings = useSettings();
  const save = useSaveSettings();
  const [glob, setGlob] = useState("");
  const list = settings.data?.protect ?? [];
  return (
    <section className="panel setting">
      <h2>Never touch</h2>
      <p className="muted">Folders and patterns ebb never offers or removes, on top of system folders, Keychains and cloud-synced folders. Use ~ for your home and ** for any depth.</p>
      <ul className="chips">
        {list.map((p) => (
          <li key={p}>
            <code>{p}</code>
            <button className="icon-btn" aria-label={`Remove ${p}`} onClick={() => save.mutate({ protect: list.filter((x) => x !== p) })}>×</button>
          </li>
        ))}
      </ul>
      <form
        className="row"
        onSubmit={(e) => {
          e.preventDefault();
          if (glob.trim()) save.mutate({ protect: [...list, glob.trim()] }, { onSuccess: () => setGlob("") });
        }}
      >
        <input className="input" value={glob} placeholder="~/Work/**" onChange={(e) => setGlob(e.target.value)} aria-label="Folder or pattern to protect" />
        <button className="btn">Protect</button>
      </form>
      {save.error && <p className="error">{(save.error as Error).message}</p>}
    </section>
  );
}

function Ignored() {
  const findings = useFindings(true);
  const ignore = useIgnore();
  const ignored = (findings.data?.findings ?? []).filter((f) => f.ignored);
  if (ignored.length === 0) return null;
  return (
    <section className="panel setting">
      <h2>Ignored findings</h2>
      <ul className="chips">
        {ignored.map((f) => (
          <li key={f.id}>
            {f.title}
            <button className="btn ghost small" onClick={() => ignore.mutate({ id: f.id, undo: true })}>Show again</button>
          </li>
        ))}
      </ul>
    </section>
  );
}

function Access() {
  const status = useStatus();
  if (!status.data) return null;
  return (
    <section className="panel setting">
      <h2>Full Disk Access</h2>
      {status.data.fullDiskAccess ? (
        <p className="muted">ebb can read app containers, Mail and Messages data.</p>
      ) : (
        <p className="muted">
          Without it, app containers and some Library folders count as “not visible”. Open{" "}
          <a href="x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles">Privacy &amp; Security → Full Disk Access</a>, add the app
          that runs ebb (your terminal), then scan again.
        </p>
      )}
    </section>
  );
}

function Web() {
  const web = useWebAccess();
  const qc = useQueryClient();
  const [code, setCode] = useState("");
  const [msg, setMsg] = useState<string | null>(null);
  const act = (path: string, body: unknown) =>
    api
      .post(path, body)
      .then(() => {
        setMsg(null);
        setCode("");
        qc.invalidateQueries({ queryKey: ["web"] });
      })
      .catch((e: ApiError) => setMsg(e.message));
  if (!web.data) return null;
  const pending = web.data.pending;
  return (
    <section className="panel setting">
      <h2>Web access</h2>
      <p className="muted">Pages that may connect to ebb on this Mac. A page gets in only after you confirm the code it shows.</p>
      <ul className="chips">
        {(web.data.allowed ?? []).map((o) => (
          <li key={o}>
            <code>{o}</code>
            {(web.data.paired ?? []).includes(o) && <span className="tag">paired</span>}
            <button className="btn ghost small" onClick={() => act("/web/deny", { origin: o })}>Remove</button>
          </li>
        ))}
      </ul>
      {pending && (
        <form className="stack pairing" onSubmit={(e) => (e.preventDefault(), act("/pair/confirm", { code }))}>
          <p>
            <code>{pending.origin}</code> wants to connect. Type the code that page shows:
          </p>
          <div className="row">
            <input className="input" inputMode="numeric" maxLength={6} value={code} onChange={(e) => setCode(e.target.value)} aria-label="Pairing code" />
            <button className="btn primary" disabled={code.length !== 6}>Confirm</button>
            <button type="button" className="btn" onClick={() => act("/pair/reject", {})}>Reject</button>
          </div>
        </form>
      )}
      {msg && <p className="error">{msg}</p>}
    </section>
  );
}

function Theme() {
  const [theme, setTheme] = useState(() => {
    try {
      return localStorage.getItem("ebb.theme") ?? "";
    } catch {
      return "";
    }
  });
  const pick = (t: string) => {
    setTheme(t);
    if (t) document.documentElement.dataset.theme = t;
    else delete document.documentElement.dataset.theme;
    try {
      if (t) localStorage.setItem("ebb.theme", t);
      else localStorage.removeItem("ebb.theme");
    } catch {}
  };
  return (
    <section className="panel setting">
      <h2>Appearance</h2>
      <div className="seg" role="group" aria-label="Theme">
        {[["", "System"], ["light", "Light"], ["dark", "Dark"]].map(([v, l]) => (
          <button key={l} aria-pressed={theme === v} onClick={() => pick(v)}>{l}</button>
        ))}
      </div>
    </section>
  );
}
