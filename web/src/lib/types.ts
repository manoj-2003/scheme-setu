// Mirrors the Go types in internal/models. Field names match the JSON tags,
// so a rename on either side shows up as a type error here rather than as a
// silently empty card.

export type Category = "general" | "obc" | "sc" | "st" | "ews" | "minority";

export type Occupation =
  | "farmer"
  | "street_vendor"
  | "artisan"
  | "micro_enterprise"
  | "self_employed"
  | "salaried"
  | "student"
  | "unemployed";

export type Purpose =
  | "working_capital"
  | "equipment"
  | "new_business"
  | "electric_vehicle"
  | "rooftop_solar"
  | "education"
  | "livestock"
  | "food_processing"
  | "income_support";

export interface Profile {
  state: string;
  district?: string;
  age: number;
  gender?: "male" | "female" | "other";
  annualIncomeInr: number;
  category: Category;
  occupation: Occupation;
  landHoldingAcres: number;
  purpose: Purpose;
  amountNeededInr: number;
  hasAadhaar: boolean;
  hasBankAccount: boolean;
  hasUdyam: boolean;
  hasDisability: boolean;
  businessVintageMonths: number;
  annualTurnoverInr: number;
  existingSchemeIds?: string[];
  language?: string;
}

export interface Benefit {
  minAmountInr?: number;
  maxAmountInr?: number;
  subsidyPercentMin?: number;
  subsidyPercentMax?: number;
  effectiveInterestPercent?: number;
  collateralFree: boolean;
  tenureMonths?: number;
  summary: string;
}

export interface Scheme {
  id: string;
  name: string;
  nameLocal?: Record<string, string>;
  shortDesc: string;
  authority: string;
  level: "central" | "state";
  states?: string[];
  kind: "loan" | "subsidy" | "credit_guarantee" | "income_support" | "insurance";
  purposes: Purpose[];
  benefit: Benefit;
  eligibility: { notes?: string[] };
  documents?: string[];
  applyUrl?: string;
  officialDomains?: string[];
  sourceUrls?: string[];
  needsVerification?: boolean;
}

export interface Reason {
  rule: string;
  passed: boolean;
  detail: string;
  blocking: boolean;
}

export interface Evidence {
  title: string;
  link: string;
  source?: string;
  date?: string;
}

export interface Freshness {
  status: "ok" | "changed" | "closed" | "unknown";
  note?: string;
  evidence?: Evidence[];
  checkedAt: string;
}

export interface Impostor {
  domain: string;
  title: string;
  link: string;
  why: string;
}

export interface TrustReport {
  applyUrlOfficial: boolean;
  applyDomain?: string;
  suspectedImpostors?: Impostor[];
  checkedAt: string;
}

export interface Match {
  scheme: Scheme;
  status: "eligible" | "near_miss" | "ineligible";
  score: number;
  reasons: Reason[];
  unlockSteps?: string[];
  freshness?: Freshness;
  trust?: TrustReport;
  origin: string;
}

export interface Lead {
  title: string;
  link: string;
  snippet?: string;
  domain: string;
  query?: string;
  language?: string;
  official: boolean;
  confidence: number;
}

export interface SearchUsage {
  calls: number;
  cacheHits: number;
  liveCalls: number;
  creditsMax: number;
}

export interface MatchResponse {
  profile: Profile;
  eligible: Match[];
  nearMisses: Match[];
  discovered: Lead[];
  usage: SearchUsage;
  warnings?: string[];
  generatedAt: string;
}

export interface Meta {
  states: string[];
  categories: Category[];
  occupations: Occupation[];
  purposes: Purpose[];
  languages: { code: string; label: string }[];
}
