import type { Lead } from "@/lib/types";

/**
 * Schemes found by search that are not in the curated catalog -- almost always
 * state schemes. Deliberately kept in a separate section and never labelled
 * "you qualify": no eligibility rules have been evaluated for these, and
 * saying otherwise about a loan would be irresponsible.
 */
export function Leads({ leads }: { leads: Lead[] }) {
  if (leads.length === 0) return null;

  return (
    <section>
      <h2 className="text-sm font-semibold tracking-wide text-zinc-400 uppercase">
        Also found on government portals ({leads.length})
      </h2>
      <p className="mt-1 max-w-2xl text-sm text-zinc-500">
        Discovered by site-restricted search across state and central
        government domains, including in your own language. These are not in
        our rule set, so eligibility has not been checked — read the source.
      </p>

      <ul className="mt-4 space-y-3">
        {leads.map((lead) => (
          <li
            key={lead.link}
            className="rounded-lg border border-zinc-800 bg-zinc-900/40 p-4"
          >
            <div className="flex flex-wrap items-baseline justify-between gap-2">
              <a
                href={lead.link}
                target="_blank"
                rel="noopener noreferrer"
                className="font-medium text-zinc-100 underline decoration-zinc-700 underline-offset-2 hover:text-accent-400"
              >
                {lead.title}
              </a>
              <span className="nums text-xs text-zinc-600">
                {Math.round(lead.confidence * 100)}% confidence
              </span>
            </div>

            {lead.snippet && (
              <p className="mt-1.5 text-sm leading-relaxed text-zinc-400">
                {lead.snippet}
              </p>
            )}

            <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-zinc-600">
              <code className="nums rounded bg-zinc-800 px-1.5 py-0.5 text-zinc-400">
                {lead.domain}
              </code>
              {lead.language && <span>found via {lead.language} search</span>}
              {lead.query && (
                <span className="truncate" title={lead.query}>
                  · {lead.query}
                </span>
              )}
            </div>
          </li>
        ))}
      </ul>
    </section>
  );
}
