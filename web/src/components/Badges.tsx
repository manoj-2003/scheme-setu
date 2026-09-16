import type { Freshness, TrustReport } from "@/lib/types";

export function Pill({
  tone,
  children,
}: {
  tone: "good" | "warn" | "bad" | "muted" | "info";
  children: React.ReactNode;
}) {
  const tones = {
    good: "bg-accent-500/15 text-accent-400 ring-accent-500/30",
    warn: "bg-amber-500/15 text-amber-300 ring-amber-500/30",
    bad: "bg-red-500/15 text-red-300 ring-red-500/30",
    muted: "bg-zinc-800 text-zinc-400 ring-zinc-700",
    info: "bg-sky-500/15 text-sky-300 ring-sky-500/30",
  } as const;

  return (
    <span
      className={`inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-medium ring-1 ring-inset ${tones[tone]}`}
    >
      {children}
    </span>
  );
}

/**
 * The verified-at stamp. This is the answer to "why not just a static
 * directory", so it is never omitted -- an unverified scheme says so.
 */
export function FreshnessBadge({ freshness }: { freshness?: Freshness }) {
  if (!freshness) return null;

  const config = {
    ok: { tone: "good", label: "Open — no change found" },
    changed: { tone: "warn", label: "Rules or dates may have moved" },
    closed: { tone: "bad", label: "May be closed — verify first" },
    unknown: { tone: "muted", label: "Not verified on this run" },
  } as const;

  const { tone, label } = config[freshness.status];
  const checked = new Date(freshness.checkedAt);

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        <Pill tone={tone}>{label}</Pill>
        <span className="text-xs text-zinc-500">
          checked {checked.toLocaleDateString("en-IN")}
        </span>
      </div>

      {freshness.note && (
        <p className="text-xs leading-relaxed text-zinc-400">{freshness.note}</p>
      )}

      {freshness.evidence && freshness.evidence.length > 0 && (
        <ul className="space-y-1">
          {freshness.evidence.map((e) => (
            <li key={e.link} className="text-xs">
              <a
                href={e.link}
                target="_blank"
                rel="noopener noreferrer"
                className="text-zinc-400 underline decoration-zinc-700 underline-offset-2 hover:text-zinc-200"
              >
                {e.title}
              </a>
              {e.source && <span className="text-zinc-600"> — {e.source}</span>}
              {e.date && <span className="text-zinc-600"> · {e.date}</span>}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

/**
 * The fraud shield. Naming the lookalike domains is the point: the people
 * worst served by scheme information are the ones least able to tell an
 * official portal from a fee-charging clone.
 */
export function TrustPanel({ trust }: { trust?: TrustReport }) {
  if (!trust) return null;

  const impostors = trust.suspectedImpostors ?? [];

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        {trust.applyUrlOfficial ? (
          <Pill tone="good">Official domain verified</Pill>
        ) : (
          <Pill tone="bad">Apply link not on an official domain</Pill>
        )}
        {trust.applyDomain && (
          <code className="nums rounded bg-zinc-800 px-1.5 py-0.5 text-xs text-zinc-400">
            {trust.applyDomain}
          </code>
        )}
      </div>

      {impostors.length > 0 && (
        <div className="rounded-lg border border-red-900/60 bg-red-950/30 p-3">
          <p className="text-sm font-semibold text-red-300">
            {impostors.length} lookalike{impostors.length > 1 ? "s" : ""} found
            in search results for this scheme
          </p>
          <p className="mt-1 text-xs text-red-200/70">
            These rank for the scheme name and invite you to apply, but they are
            not government sites. Never pay a fee to apply for a government
            scheme.
          </p>
          <ul className="mt-2 space-y-2">
            {impostors.map((i) => (
              <li key={i.link} className="text-xs">
                <code className="nums text-red-300">{i.domain}</code>
                <p className="mt-0.5 text-zinc-400">{i.why}</p>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
