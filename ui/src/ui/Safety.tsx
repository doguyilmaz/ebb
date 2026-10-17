import { CircleAlert, CircleCheck, OctagonX, UserRound } from "lucide-react";

const meta = {
  regenerable: { icon: CircleCheck, label: "Rebuilds itself" },
  caution: { icon: CircleAlert, label: "Check first" },
  "user-data": { icon: UserRound, label: "Your data" },
  system: { icon: OctagonX, label: "System" },
} as const;

export type SafetyLevel = keyof typeof meta;

export function Safety({ level }: { level: string }) {
  const m = meta[level as SafetyLevel] ?? meta.caution;
  const Icon = m.icon;
  return (
    <span className={`safety ${level}`} data-term={`safety.${level in meta ? level : "caution"}`}>
      <Icon size={12} aria-hidden />
      {m.label}
    </span>
  );
}
