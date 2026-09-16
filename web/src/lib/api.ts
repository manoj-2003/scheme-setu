import type { MatchResponse, Meta, Profile } from "./types";

const API_BASE = process.env.NEXT_PUBLIC_API_BASE ?? "http://localhost:8080";

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let res: Response;
  try {
    res = await fetch(`${API_BASE}${path}`, {
      ...init,
      headers: { "content-type": "application/json", ...init?.headers },
    });
  } catch {
    // The most common failure by far is the Go server not running, so say
    // that rather than surfacing a bare "Failed to fetch".
    throw new Error(
      `Cannot reach the Scheme Setu API at ${API_BASE}. Start it with: go run ./cmd/server`,
    );
  }

  if (!res.ok) {
    const body = await res.text();
    let message = body;
    try {
      const parsed = JSON.parse(body) as { message?: string };
      if (parsed.message) message = parsed.message;
    } catch {
      // Not JSON; the raw body is the best message available.
    }
    throw new Error(message || `Request failed with ${res.status}`);
  }

  return res.json() as Promise<T>;
}

export const fetchMeta = () => request<Meta>("/api/v1/meta");

export const matchProfile = (profile: Profile) =>
  request<MatchResponse>("/api/v1/match", {
    method: "POST",
    body: JSON.stringify(profile),
  });

/**
 * Formats an amount using the Indian grouping system, because "₹10,00,000"
 * reads correctly to the person this is built for and "₹1,000,000" does not.
 */
export function rupees(n: number): string {
  return `₹${n.toLocaleString("en-IN", { maximumFractionDigits: 0 })}`;
}

/** Renders large amounts the way people say them out loud. */
export function rupeesShort(n: number): string {
  if (n >= 10_000_000) return `₹${trim(n / 10_000_000)} crore`;
  if (n >= 100_000) return `₹${trim(n / 100_000)} lakh`;
  return rupees(n);
}

const trim = (n: number) => Number(n.toFixed(2)).toString();

export const titleCase = (slug: string) =>
  slug
    .split("_")
    .map((w) => (w === "and" ? w : w.charAt(0).toUpperCase() + w.slice(1)))
    .join(" ");
