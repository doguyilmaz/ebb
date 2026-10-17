import type { ReactNode } from "react";

/** Every page says what it is for in one line, so nobody has to guess. */
export function PageHead({ title, sub, children }: { title: ReactNode; sub?: ReactNode; children?: ReactNode }) {
  return (
    <header className="page-head">
      <div className="stack" style={{ gap: 4 }}>
        <h1>{title}</h1>
        {sub && <p className="page-sub">{sub}</p>}
      </div>
      {children && <div className="row page-actions">{children}</div>}
    </header>
  );
}
