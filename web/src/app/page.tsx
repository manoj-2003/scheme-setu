"use client";

import { useState } from "react";

import { Leads } from "@/components/Leads";
import { SchemeCard } from "@/components/SchemeCard";
import { Wizard } from "@/components/Wizard";
import { matchProfile } from "@/lib/api";
import type { MatchResponse, Profile } from "@/lib/types";

export default function Home() {
  const [result, setResult] = useState<MatchResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function run(profile: Profile) {
    setLoading(true);
    setError(null);
    try {
      setResult(await matchProfile(profile));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Something went wrong");
      setResult(null);
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="mx-auto max-w-6xl px-6 py-12">
      <header className="max-w-3xl">
        <h1 className="text-3xl font-bold tracking-tight text-zinc-50">
          Scheme Setu
        </h1>
        <p className="mt-3 text-lg leading-relaxed text-zinc-300">
          Tell us six things about yourself. See every government loan and
          subsidy you actually qualify for — with the real application link, the
          documents you need, proof it is still open today, and a warning about
          the fake portals ranking above it.
        </p>
      </header>

      <div className="mt-10 grid gap-10 lg:grid-cols-[minmax(0,26rem)_minmax(0,1fr)]">
        <div className="lg:sticky lg:top-12 lg:self-start">
          <Wizard onSubmit={run} loading={loading} />
        </div>

        <div className="space-y-10">
          {error && (
            <div className="rounded-xl border border-red-900/60 bg-red-950/30 p-5">
              <p className="font-medium text-red-300">{error}</p>
            </div>
          )}

          {!result && !error && (
            <div className="rounded-xl border border-dashed border-zinc-800 p-8">
              <p className="text-sm leading-relaxed text-zinc-500">
                Answer the six questions and your matches appear here. Each card
                shows the money first, then every eligibility rule we checked,
                then whether the scheme is still open — verified against news
                coverage, not just a stored deadline.
              </p>
            </div>
          )}

          {result && (
            <>
              <UsageBar result={result} />

              <section>
                <h2 className="text-sm font-semibold tracking-wide text-zinc-400 uppercase">
                  You qualify for {result.eligible.length}
                </h2>
                {result.eligible.length === 0 ? (
                  <p className="mt-3 text-sm text-zinc-500">
                    Nothing in the catalog matches this profile outright. Check
                    the near misses below — most are one document away.
                  </p>
                ) : (
                  <div className="mt-4 space-y-4">
                    {result.eligible.map((m) => (
                      <SchemeCard key={m.scheme.id} match={m} />
                    ))}
                  </div>
                )}
              </section>

              {result.nearMisses.length > 0 && (
                <section>
                  <h2 className="text-sm font-semibold tracking-wide text-zinc-400 uppercase">
                    One step away ({result.nearMisses.length})
                  </h2>
                  <p className="mt-1 max-w-2xl text-sm text-zinc-500">
                    You do not qualify today, but each of these is blocked only
                    by something you can fix.
                  </p>
                  <div className="mt-4 space-y-4">
                    {result.nearMisses.map((m) => (
                      <SchemeCard key={m.scheme.id} match={m} />
                    ))}
                  </div>
                </section>
              )}

              <Leads leads={result.discovered} />

              {result.warnings && result.warnings.length > 0 && (
                <details className="rounded-xl border border-zinc-800 bg-zinc-900/40 p-5">
                  <summary className="cursor-pointer text-sm text-zinc-400">
                    {result.warnings.length} note
                    {result.warnings.length > 1 ? "s" : ""} about this run
                  </summary>
                  <ul className="mt-3 space-y-1">
                    {result.warnings.map((w) => (
                      <li key={w} className="text-xs text-zinc-500">
                        {w}
                      </li>
                    ))}
                  </ul>
                </details>
              )}
            </>
          )}
        </div>
      </div>
    </main>
  );
}

/**
 * Shows what the request cost. The free SerpApi plan is 250 credits a month,
 * so proving the run came from cache is part of the story, not a footnote.
 */
function UsageBar({ result }: { result: MatchResponse }) {
  const { usage } = result;
  const stats = [
    { label: "searches", value: usage.calls },
    { label: "from cache", value: usage.cacheHits },
    { label: "credits spent", value: usage.liveCalls },
    { label: "ceiling", value: usage.creditsMax },
  ];

  return (
    <div className="flex flex-wrap items-center gap-x-6 gap-y-2 rounded-xl border border-zinc-800 bg-zinc-900/40 px-5 py-3">
      {stats.map((s) => (
        <div key={s.label} className="flex items-baseline gap-1.5">
          <span className="nums text-lg font-semibold text-zinc-100">
            {s.value}
          </span>
          <span className="text-xs text-zinc-500">{s.label}</span>
        </div>
      ))}
      <span className="ml-auto text-xs text-zinc-600">
        {new Date(result.generatedAt).toLocaleString("en-IN")}
      </span>
    </div>
  );
}
