# Scheme Setu

**Tell us six things about yourself. Get every government loan and subsidy you actually qualify for — with the real application link, the documents you need, proof it is still open today, and a warning about the fake portals ranking above it.**

Built for the [SerpApi India Hackathon 2026](https://serpapi.com/).

India runs hundreds of credit and subsidy schemes — PM MUDRA, PM SVANidhi, PMEGP,
Kisan Credit Card, Stand-Up India, PM Vishwakarma, state EV and MSME subsidies. The
money is real and largely unclaimed, because the information is scattered across
thirty-odd state portals, mostly as PDF circulars, often only in the local language,
and the search results for any scheme name are crowded with lookalike sites that charge
a fee to fill a free form.

Scheme Setu is a matching engine over that mess.

---

## Why search data is the product, not a garnish

A curated rule set alone would be a static directory — useful for a month, wrong by
the next budget. Live search alone would be irresponsible: you cannot infer "you
qualify for ₹10 lakh" from a search snippet and hand that to someone deciding whether
to borrow. So the two are split by what each is actually good at:

| Layer | Source | Job |
|---|---|---|
| **1. Eligibility** | Curated rules in [`internal/catalog/schemes.json`](internal/catalog/schemes.json) | Decide *if you qualify*, and explain every rule it applied |
| **2. Discovery** | SerpApi, site-restricted, multilingual | Find the **state** schemes no directory lists |
| **3. Freshness** | SerpApi (Google News) | Answer *is this still open today?* — stamp every card |
| **4. Fraud shield** | SerpApi (Google Search) | Verify the apply link is official, and **name the impostors** ranking above it |

Remove SerpApi and layers 2–4 vanish: you are left with a spreadsheet that goes stale
and sends people to fee-charging lookalikes. That is the test this project was designed
to pass.

## The four things that make it more than a directory

**1. Explainable matching.** Every card lists the rules evaluated and whether each
passed. Nobody should take a loan decision from an opaque yes/no.

```
PM SVANidhi — First Tranche                          ELIGIBLE  ·  score 88
  ✓ occupation   For street vendor (you selected Street vendor)
  ✓ age          Minimum age 18 (you are 34)
  ✓ aadhaar      Aadhaar is mandatory for this scheme
  ✓ bank_account Disbursal is direct-to-account, so a bank account is required
```

**2. Near misses with unlock steps.** Often more valuable than the matches. Rules are
tagged *blocking* (age, category — nothing you can do today) or *fixable*, and every
fixable gap becomes an instruction:

```
PM MUDRA Yojana — Tarun                              NEAR MISS  ·  score 68
  ✗ udyam        Requires Udyam (MSME) registration
  → Register free on the Udyam portal (udyamregistration.gov.in). It takes minutes
    with Aadhaar and PAN, and unlocks several MSME schemes at once.
```

**3. The fraud shield.** For each top match, Scheme Setu runs the query a worried
applicant would actually type — `"PM SVANidhi" apply online registration` — and flags
every result that solicits an application from a domain that is not `gov.in` / `nic.in`
or a recognised institution. Lookalike domains never inherit trust by embedding a
government string in their name; that is [tested](internal/fraud/fraud_test.go).

**4. Multilingual discovery.** The same site-restricted queries run with `hl=ta`, `hi`,
`bn`, `mr`. This is not UI translation — state portals *publish* in the local language,
so the Tamil query genuinely returns circulars the English one never surfaces.

---

## Architecture

```
POST /api/v1/match
        │
        ├─ catalog.Candidates(profile)      narrow by state + purpose  (0 credits)
        ├─ rules.EvaluateAll(...)           eligible / near-miss / out  (0 credits)
        │
        └─ errgroup fan-out ───┬─ freshness.CheckAll    google_news
                               ├─ fraud.Check           google
                               └─ discovery.Run         google + autocomplete,
                                                        site-restricted, per language
```

Go was chosen for exactly that fan-out: one submission issues 20–30 independent SerpApi
calls across engines and languages, which is `errgroup` plus a bounded worker limit
rather than an async framework.

```
cmd/server        Echo API + graceful shutdown
cmd/warm          pre-fetches the demo cache, then the app runs offline
internal/serp     credit-aware SerpApi client: SQLite cache, call ceiling, cache-only mode
internal/catalog  embedded curated rule set (//go:embed)
internal/rules    eligibility engine + explanations + ranking
internal/discovery query planner (site:, filetype:pdf, per-language) + lead scoring
internal/freshness news verification, "is it still open"
internal/fraud    official-domain verification + impostor detection
```

## Credit discipline

The free SerpApi plan grants **250 credits a month** — about eight searches a day — and
each results page costs one. The whole client is built around that:

- **Every response is cached** to SQLite, keyed by a hash of the request parameters
  (minus the API key), so no query is ever paid for twice.
- **A hard per-process ceiling** (`SERP_MAX_CREDITS`, default 40) stops a runaway loop
  from eating the month.
- **The catalog narrows before anything is spent** — state, then purpose — so a profile
  only triggers the queries that could possibly match.
- **Cache-only mode** (`SERP_MODE=cache`) never touches the network. `cmd/warm`
  pre-fetches the ten demo personas once; after that the app answers instantly,
  offline, for zero credits.
- **`GET /api/v1/usage`** reports calls, cache hits and live credits spent, so the
  demo can prove it is running off the committed cache.
- Unverified results are **labelled**, never silently presented as verified.

The pure-Go `modernc.org/sqlite` driver is deliberate: no CGO, no C toolchain, so
`go build` works on a clean Windows machine.

---

## Running it

### Without a SerpApi key (works immediately)

```bash
git clone https://github.com/manoj-2003/scheme-setu
cd scheme-setu
SERP_MODE=cache SERP_CACHE_PATH=fixtures/serp-cache.sqlite go run ./cmd/server
```

Eligibility matching, explanations, unlock steps and apply-link verification are fully
offline and need no key. Discovery, news freshness and impostor detection serve from
the committed cache and are labelled unverified on a miss.

### With a key (full pipeline)

```bash
cp .env.example .env     # add SERPAPI_KEY from https://serpapi.com/
go run ./cmd/server
```

### Warm the demo cache before recording

```bash
go run ./cmd/warm -dry-run      # show the queries and the credit cost first
go run ./cmd/warm               # fetch them into fixtures/serp-cache.sqlite
```

Then commit the cache file and run the server with `SERP_MODE=cache`.

### Tests

```bash
go test ./...
```

## API

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/api/v1/match` | Profile in, ranked + verified schemes out |
| `GET` | `/api/v1/schemes` | The whole catalog |
| `GET` | `/api/v1/meta` | Enumerations for the wizard dropdowns |
| `GET` | `/api/v1/usage` | SerpApi calls, cache hits, credits spent |
| `GET` | `/healthz` | Liveness |

```bash
curl -s localhost:8080/api/v1/match -H 'content-type: application/json' -d '{
  "state": "tamil_nadu", "district": "Coimbatore",
  "age": 34, "gender": "female", "category": "obc",
  "occupation": "street_vendor", "annualIncomeInr": 180000,
  "purpose": "working_capital", "amountNeededInr": 50000,
  "hasAadhaar": true, "hasBankAccount": true,
  "businessVintageMonths": 30, "language": "ta"
}'
```

## SerpApi engines used

| Engine | Used for |
|---|---|
| `google` | Site-restricted state-portal discovery (`site:tn.gov.in`), guidelines PDFs (`filetype:pdf`), impostor detection |
| `google_news` | Freshness verification — extended, revised, suspended, closed |
| `google_autocomplete` | What applicants actually type, surfaced as related questions and used to seed better queries |

## Catalog status

Catalog entries currently carry `needsVerification: true`. The amounts and rules are
drawn from the schemes as publicly documented, but **every figure must be checked
against its official source before this is put in front of real applicants**, and the
flag exists so the UI can say so rather than quietly implying authority it has not
earned. `GET /api/v1/schemes` returns the unverified list, and the server logs a
warning at startup while any remain.

Scheme Setu is an information tool. It does not apply on anyone's behalf, never asks
for a fee, and points only at official portals.

## Licence

MIT
