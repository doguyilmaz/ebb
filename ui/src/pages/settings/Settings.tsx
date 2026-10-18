import { useQueryClient } from "@tanstack/react-query";
import { FolderOpen, Plus, X } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";
import { api, ApiError } from "#/api/client.ts";
import { useDaemon, useDaemonAction, useFindings, useIgnore, useSaveSettings, useSettings, useStatus, useWebAccess } from "#/api/hooks.ts";
import type { ChooseResponse, Settings as S } from "#/api/types.gen.ts";
import { ago } from "#/lib/format.ts";
import type { TermId } from "#/lib/glossary.ts";
import { PageHead } from "#/ui/PageHead.tsx";
import { Loading, Problem } from "#/ui/states.tsx";
import { Info } from "#/ui/Tip.tsx";

function Head({ title, term }: { title: string; term?: TermId }) {
  return (
    <div className="setting-head">
      <h2>{title}</h2>
      {term && <Info term={term} />}
    </div>
  );
}

export default function Settings() {
  return (
    <>
      <PageHead title="Settings" sub="How ebb scans, what it never touches, and which web pages may connect to it." />
      <Scan />
      <Protect />
      <Background />
      <Ignored />
      <Access />
      <Web />
      <Theme />
    </>
  );
}

function useChooseFolder() {
  const [error, setError] = useState<string | null>(null);
  const choose = async (purpose: "scan" | "protect"): Promise<string> => {
    setError(null);
    try {
      return (await api.post<ChooseResponse>("/choose-folder", { purpose })).path;
    } catch (e) {
      setError((e as Error).message);
      return "";
    }
  };
  return { choose, error };
}

function Field({ label, help, children }: { label: string; help: string; children: ReactNode }) {
  return (
    <label className="field">
      <span>{label}</span>
      {children}
      <span className="field-help">{help}</span>
    </label>
  );
}

function Scan() {
  const settings = useSettings();
  const save = useSaveSettings();
  const picker = useChooseFolder();
  const [draft, setDraft] = useState<S | null>(null);
  useEffect(() => setDraft(settings.data ?? null), [settings.data]);
  if (settings.error) return <Problem error={settings.error} />;
  if (!draft) return <Loading />;
  const set = <K extends keyof S>(k: K, v: S[K]) => setDraft({ ...draft, [k]: v });
  return (
    <section className="card setting">
      <Head title="Scanning" />
      <Field label="Folder to scan" help="Empty scans the whole disk. Type a path or pick one.">
        <span className="row">
          <input className="input grow" value={draft.root} placeholder="/System/Volumes/Data" onChange={(e) => set("root", e.target.value)} />
          <button
            type="button"
            className="btn"
            onClick={async (e) => {
              e.preventDefault();
              const p = await picker.choose("scan");
              if (p) set("root", p);
            }}
          >
            <FolderOpen size={14} aria-hidden /> Browse…
          </button>
        </span>
      </Field>
      {picker.error && <p className="error small-text">{picker.error}</p>}
      <div className="fields">
        <Field label="Parallel readers" help="Folders read at once. Empty picks what suits this Mac.">
          <input className="input" type="number" min={0} max={64} placeholder="Automatic" value={draft.threads || ""} onChange={(e) => set("threads", Number(e.target.value) || 0)} />
        </Field>
        <Field label="Untouched after" help="Days without changes before a project counts as untouched.">
          <span className="input-unit">
            <input className="input" type="number" min={1} max={3650} value={draft.staleDays} onChange={(e) => set("staleDays", Number(e.target.value))} />
            <span className="faint">days</span>
          </span>
        </Field>
        <Field label="Large file from" help="Files at least this big are listed, like 500 MB or 2 GB.">
          <input className="input" value={draft.largeFile} onChange={(e) => set("largeFile", e.target.value)} />
        </Field>
      </div>
      <div className="row">
        <button className="btn primary" disabled={save.isPending} onClick={() => save.mutate(draft)}>Save</button>
        {save.error && <span className="error">{(save.error as Error).message}</span>}
        {save.isSuccess && <span className="muted">Saved. Changes apply to the next scan.</span>}
      </div>
    </section>
  );
}

const suggestions = [
  { glob: "~/Documents/**", label: "Documents" },
  { glob: "~/Desktop/**", label: "Desktop" },
  { glob: "~/Pictures/**", label: "Pictures" },
  { glob: "~/Movies/**", label: "Movies" },
  { glob: "~/Music/**", label: "Music" },
];

function Protect() {
  const settings = useSettings();
  const save = useSaveSettings();
  const picker = useChooseFolder();
  const [glob, setGlob] = useState("");
  const list = settings.data?.protect ?? [];
  const add = (g: string) => g && !list.includes(g) && save.mutate({ protect: [...list, g] }, { onSuccess: () => setGlob("") });
  const offered = suggestions.filter((sg) => !list.includes(sg.glob));
  return (
    <section className="card setting">
      <Head title="Never touch" term="protect" />
      <p className="muted">
        Folders ebb never offers or removes, on top of system folders, Keychains and cloud-synced folders. Use ~ for your home and ** for everything
        inside.
      </p>
      {list.length > 0 && (
        <ul className="chips">
          {list.map((p) => (
            <li key={p}>
              <code>{p}</code>
              <button className="icon-btn" aria-label={`Stop protecting ${p}`} data-tip="Stop protecting" onClick={() => save.mutate({ protect: list.filter((x) => x !== p) })}>
                <X size={14} />
              </button>
            </li>
          ))}
        </ul>
      )}
      <form
        className="row"
        onSubmit={(e) => {
          e.preventDefault();
          add(glob.trim());
        }}
      >
        <input className="input grow" value={glob} placeholder="~/Work/**" onChange={(e) => setGlob(e.target.value)} aria-label="Folder or pattern to protect" />
        <button
          type="button"
          className="btn"
          onClick={async () => {
            const p = await picker.choose("protect");
            if (p) setGlob(`${p}/**`);
          }}
        >
          <FolderOpen size={14} aria-hidden /> Browse…
        </button>
        <button className="btn primary" disabled={!glob.trim()}>Protect</button>
      </form>
      {offered.length > 0 && (
        <div className="suggest">
          <span className="faint small-text">Suggestions</span>
          {offered.map((sg) => (
            <button key={sg.glob} className="btn small" onClick={() => add(sg.glob)} data-tip={`Protects everything in ${sg.glob.replace("/**", "")}`}>
              <Plus size={13} aria-hidden /> {sg.label}
            </button>
          ))}
        </div>
      )}
      {(save.error || picker.error) && <p className="error">{((save.error as Error) ?? null)?.message ?? picker.error}</p>}
    </section>
  );
}

/** Go writes a time never set as year 1. */
const when = (iso: string) => (new Date(iso).getFullYear() > 1 ? ago(iso) : null);

function Background() {
  const daemon = useDaemon();
  const settings = useSettings();
  const save = useSaveSettings();
  const act = useDaemonAction();
  const [gb, setGb] = useState<string | null>(null);
  const d = daemon.data;
  const s = settings.data;
  if (!d?.supported || !s) return null;
  const state = d.waiting
    ? "Installed. It takes over when the ebb serving this window stops."
    : d.running
      ? `Running since ${when(d.started) ?? "now"}.`
      : d.installed
        ? "Installed but not running. Restart it, or see the log."
        : "Off. ebb runs only while you start it.";
  const scanned = when(d.lastWorkAt);
  const commitGb = () => {
    if (gb === null) return;
    const n = Math.max(0, Math.round(Number(gb) || 0));
    setGb(null);
    if (n !== s.lowSpaceGb) save.mutate({ lowSpaceGb: n });
  };
  return (
    <section className="card setting">
      <Head title="Background service" term="daemon" />
      <p className="muted">
        Starts at login and keeps this window ready. While the Mac is on power and nobody uses it, it updates the last scan, at most every half hour
        and in full once a week, and warns once when free space runs low. It never cleans anything on its own.
      </p>
      <p className="service-state">
        <span className="led" data-on={d.running && !d.waiting} aria-hidden />
        {state}
        {scanned && <span className="faint"> Last background scan {scanned}.</span>}
      </p>
      {d.error && <p className="error small-text">{d.error}</p>}
      <div className="fields">
        <div className="field">
          <span>Keep the last scan fresh</span>
          <button
            role="switch"
            aria-checked={s.backgroundScans}
            className="switch"
            onClick={() => save.mutate({ backgroundScans: !s.backgroundScans })}
            aria-label="Keep the last scan fresh"
          />
          <span className="field-help">Off: it serves the window and warns, but never scans by itself.</span>
        </div>
        <Field label="Warn below" help="Free space that triggers one notification. 0 never warns.">
          <span className="input-unit">
            <input
              className="input"
              type="number"
              min={0}
              value={gb ?? s.lowSpaceGb}
              onChange={(e) => setGb(e.target.value)}
              onBlur={commitGb}
              onKeyDown={(e) => e.key === "Enter" && commitGb()}
            />
            <span className="faint">GB</span>
          </span>
        </Field>
      </div>
      <div className="row">
        {d.installed ? (
          <>
            <button className="btn" disabled={act.isPending} onClick={() => act.mutate("restart")}>Restart</button>
            <button className="btn ghost" disabled={act.isPending} onClick={() => act.mutate("uninstall")}>Turn off</button>
          </>
        ) : (
          <button className="btn primary" disabled={act.isPending} onClick={() => act.mutate("install")}>Run in the background</button>
        )}
        {(act.error || save.error) && <span className="error">{((act.error ?? save.error) as Error).message}</span>}
      </div>
    </section>
  );
}

function Ignored() {
  const findings = useFindings(true);
  const ignore = useIgnore();
  const ignored = (findings.data?.findings ?? []).filter((f) => f.ignored);
  if (ignored.length === 0) return null;
  return (
    <section className="card setting">
      <Head title="Ignored findings" term="ignore" />
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
    <section className="card setting">
      <Head title="Full Disk Access" term="fda" />
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
    <section className="card setting">
      <Head title="Web access" term="web" />
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
    <section className="card setting">
      <Head title="Appearance" />
      <div className="seg" role="group" aria-label="Theme">
        {[["", "System"], ["light", "Light"], ["dark", "Dark"]].map(([v, l]) => (
          <button key={l} aria-pressed={theme === v} onClick={() => pick(v)}>{l}</button>
        ))}
      </div>
    </section>
  );
}
