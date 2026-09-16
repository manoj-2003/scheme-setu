"use client";

import { useState } from "react";

import { rupeesShort } from "@/lib/api";
import type { Match } from "@/lib/types";

import { FreshnessBadge, Pill, TrustPanel } from "./Badges";

/** The money line. Cards lead with this, because it is what makes someone read on. */
function headline(m: Match): string {
  const b = m.scheme.benefit;

  if (b.maxAmountInr && b.minAmountInr) {
    return `${rupeesShort(b.minAmountInr)} – ${rupeesShort(b.maxAmountInr)}`;
  }
  if (b.maxAmountInr) return `Up to ${rupeesShort(b.maxAmountInr)}`;
  if (b.subsidyPercentMax) return `${b.subsidyPercentMax}% subsidy`;
  return "Incentive at purchase";
}

export function SchemeCard({ match }: { match: Match }) {
  const [showRules, setShowRules] = useState(false);
  const { scheme, status, reasons, unlockSteps } = match;

  const eligible = status === "eligible";
  const failed = reasons.filter((r) => !r.passed);

  return (
    <article className="rounded-xl border border-zinc-800 bg-zinc-900/60 p-5">
      <header className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="text-base font-semibold text-zinc-50">
              {scheme.name}
            </h3>
            <Pill tone={eligible ? "good" : "warn"}>
              {eligible ? "You qualify" : "One step away"}
            </Pill>
            {scheme.level === "state" && <Pill tone="info">State scheme</Pill>}
          </div>
          <p className="mt-1 text-sm text-zinc-400">{scheme.shortDesc}</p>
          <p className="mt-0.5 text-xs text-zinc-600">{scheme.authority}</p>
        </div>

        <div className="text-right">
          <div className="nums text-lg font-semibold text-accent-400">
            {headline(match)}
          </div>
          {scheme.benefit.collateralFree && (
            <div className="text-xs text-zinc-500">no collateral</div>
          )}
          {!!scheme.benefit.effectiveInterestPercent && (
            <div className="nums text-xs text-zinc-500">
              {scheme.benefit.effectiveInterestPercent}% effective interest
            </div>
          )}
        </div>
      </header>

      <p className="mt-3 text-sm leading-relaxed text-zinc-300">
        {scheme.benefit.summary}
      </p>

      {/* Near misses lead with the fix, not the rejection. */}
      {!eligible && unlockSteps && unlockSteps.length > 0 && (
        <div className="mt-4 rounded-lg border border-amber-900/60 bg-amber-950/20 p-3">
          <p className="text-xs font-semibold tracking-wide text-amber-300 uppercase">
            To unlock this
          </p>
          <ul className="mt-2 space-y-2">
            {unlockSteps.map((s) => (
              <li key={s} className="text-sm leading-relaxed text-amber-100/90">
                {s}
              </li>
            ))}
          </ul>
          {failed.length > 0 && (
            <p className="mt-2 text-xs text-amber-200/60">
              Blocked by: {failed.map((r) => r.rule).join(", ")}
            </p>
          )}
        </div>
      )}

      <div className="mt-4 space-y-3 border-t border-zinc-800 pt-4">
        <FreshnessBadge freshness={match.freshness} />
        <TrustPanel trust={match.trust} />
      </div>

      <button
        type="button"
        onClick={() => setShowRules((v) => !v)}
        className="mt-4 text-xs text-zinc-400 underline decoration-zinc-700 underline-offset-2 hover:text-zinc-200"
      >
        {showRules ? "Hide" : "Show"} the {reasons.length} rules we checked
      </button>

      {showRules && (
        <ul className="mt-3 space-y-1.5">
          {reasons.map((r) => (
            <li key={r.rule + r.detail} className="flex gap-2 text-sm">
              <span
                aria-hidden
                className={r.passed ? "text-accent-400" : "text-amber-400"}
              >
                {r.passed ? "✓" : "✗"}
              </span>
              <span className="sr-only">{r.passed ? "Passed" : "Failed"}</span>
              <span className="text-zinc-400">
                <code className="text-xs text-zinc-500">{r.rule}</code>{" "}
                {r.detail}
              </span>
            </li>
          ))}
        </ul>
      )}

      {scheme.documents && scheme.documents.length > 0 && (
        <div className="mt-4">
          <p className="text-xs font-semibold tracking-wide text-zinc-500 uppercase">
            Documents to carry
          </p>
          <div className="mt-2 flex flex-wrap gap-1.5">
            {scheme.documents.map((d) => (
              <span
                key={d}
                className="rounded bg-zinc-800 px-2 py-0.5 text-xs text-zinc-300"
              >
                {d}
              </span>
            ))}
          </div>
        </div>
      )}

      {scheme.eligibility.notes && scheme.eligibility.notes.length > 0 && (
        <ul className="mt-4 space-y-1">
          {scheme.eligibility.notes.map((n) => (
            <li key={n} className="text-xs leading-relaxed text-zinc-500">
              — {n}
            </li>
          ))}
        </ul>
      )}

      <footer className="mt-5 flex flex-wrap items-center gap-3">
        {scheme.applyUrl && (
          <a
            href={scheme.applyUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="rounded-lg bg-accent-600 px-3.5 py-2 text-sm font-medium text-zinc-950 transition hover:bg-accent-500"
          >
            Apply on the official portal →
          </a>
        )}
        {scheme.needsVerification && (
          <span className="text-xs text-zinc-600">
            Amounts not yet verified against the official source
          </span>
        )}
      </footer>
    </article>
  );
}
