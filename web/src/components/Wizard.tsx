"use client";

import { useEffect, useState } from "react";

import { fetchMeta, rupeesShort, titleCase } from "@/lib/api";
import type { Category, Meta, Occupation, Profile, Purpose } from "@/lib/types";

const OCCUPATIONS: Record<Occupation, string> = {
  farmer: "Farmer",
  street_vendor: "Street vendor / hawker",
  artisan: "Artisan / weaver / craftsperson",
  micro_enterprise: "Shop or small business owner",
  self_employed: "Self-employed / gig worker",
  salaried: "Salaried employee",
  student: "Student",
  unemployed: "Not currently working",
};

const PURPOSES: Record<Purpose, string> = {
  working_capital: "Working capital for my business",
  equipment: "Tools, machinery or equipment",
  new_business: "Start a new business",
  electric_vehicle: "Buy an electric vehicle",
  rooftop_solar: "Rooftop solar for my home",
  education: "Education fees",
  livestock: "Dairy, poultry or livestock",
  food_processing: "A food processing unit",
  income_support: "Direct income support",
};

const CATEGORIES: Record<Category, string> = {
  general: "General",
  obc: "OBC",
  sc: "Scheduled Caste (SC)",
  st: "Scheduled Tribe (ST)",
  ews: "EWS",
  minority: "Minority",
};

const FALLBACK_STATES = [
  "andhra_pradesh", "assam", "bihar", "chhattisgarh", "delhi", "gujarat",
  "haryana", "jharkhand", "karnataka", "kerala", "madhya_pradesh",
  "maharashtra", "odisha", "punjab", "rajasthan", "tamil_nadu", "telangana",
  "uttar_pradesh", "uttarakhand", "west_bengal",
];

const FALLBACK_LANGUAGES = [
  { code: "en", label: "English" },
  { code: "hi", label: "हिन्दी" },
  { code: "ta", label: "தமிழ்" },
  { code: "te", label: "తెలుగు" },
  { code: "bn", label: "বাংলা" },
  { code: "mr", label: "मराठी" },
];

const BUSINESS_OCCUPATIONS: Occupation[] = [
  "micro_enterprise",
  "self_employed",
  "street_vendor",
  "artisan",
];

const INITIAL: Profile = {
  state: "tamil_nadu",
  district: "",
  age: 30,
  gender: "female",
  annualIncomeInr: 200000,
  category: "obc",
  occupation: "street_vendor",
  landHoldingAcres: 0,
  purpose: "working_capital",
  amountNeededInr: 50000,
  hasAadhaar: true,
  hasBankAccount: true,
  hasUdyam: false,
  hasDisability: false,
  businessVintageMonths: 24,
  annualTurnoverInr: 400000,
  language: "ta",
};

const STEPS = [
  "Where you live",
  "About you",
  "What you do",
  "What the money is for",
  "What you already have",
  "Search language",
];

export function Wizard({
  onSubmit,
  loading,
}: {
  onSubmit: (p: Profile) => void;
  loading: boolean;
}) {
  const [profile, setProfile] = useState<Profile>(INITIAL);
  const [step, setStep] = useState(0);
  const [meta, setMeta] = useState<Meta | null>(null);

  // The API owns these enumerations so the wizard can never drift from the
  // Go types. A fetch failure is non-fatal: fall back to a built-in list.
  useEffect(() => {
    fetchMeta()
      .then(setMeta)
      .catch(() => setMeta(null));
  }, []);

  const states = meta?.states?.length ? [...meta.states].sort() : FALLBACK_STATES;
  const languages = meta?.languages?.length ? meta.languages : FALLBACK_LANGUAGES;

  const set = <K extends keyof Profile>(key: K, value: Profile[K]) =>
    setProfile((p) => ({ ...p, [key]: value }));

  const isFarmer = profile.occupation === "farmer";
  const runsBusiness = BUSINESS_OCCUPATIONS.includes(profile.occupation);
  const last = step === STEPS.length - 1;

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        if (last) onSubmit(profile);
        else setStep((s) => s + 1);
      }}
      className="rounded-xl border border-zinc-800 bg-zinc-900/60 p-6"
    >
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold text-zinc-200">
          {step + 1}. {STEPS[step]}
        </h2>
        <span className="nums text-xs text-zinc-500">
          step {step + 1} of {STEPS.length}
        </span>
      </div>

      <div className="mt-2 flex gap-1" aria-hidden>
        {STEPS.map((s, i) => (
          <div
            key={s}
            className={`h-1 flex-1 rounded-full ${i <= step ? "bg-accent-500" : "bg-zinc-800"}`}
          />
        ))}
      </div>

      <div className="mt-6 space-y-5">
        {step === 0 && (
          <>
            <Select
              label="State"
              value={profile.state}
              onChange={(v) => set("state", v)}
              options={states.map((s) => ({ value: s, label: titleCase(s) }))}
            />
            <Text
              label="District or city"
              hint="Optional, but it makes the results more local"
              value={profile.district ?? ""}
              onChange={(v) => set("district", v)}
            />
          </>
        )}

        {step === 1 && (
          <>
            <Number
              label="Age"
              value={profile.age}
              min={16}
              max={100}
              onChange={(v) => set("age", v)}
            />
            <Radio
              label="Gender"
              value={profile.gender ?? "female"}
              onChange={(v) => set("gender", v as Profile["gender"])}
              options={[
                { value: "female", label: "Female" },
                { value: "male", label: "Male" },
                { value: "other", label: "Other" },
              ]}
              hint="Several schemes reserve larger amounts or lower interest for women."
            />
            <Select
              label="Category"
              value={profile.category}
              onChange={(v) => set("category", v as Category)}
              options={Object.entries(CATEGORIES).map(([value, label]) => ({
                value,
                label,
              }))}
            />
          </>
        )}

        {step === 2 && (
          <>
            <Select
              label="What best describes your work?"
              value={profile.occupation}
              onChange={(v) => set("occupation", v as Occupation)}
              options={Object.entries(OCCUPATIONS).map(([value, label]) => ({
                value,
                label,
              }))}
            />
            {isFarmer && (
              <Number
                label="Land you hold (acres)"
                value={profile.landHoldingAcres}
                step={0.5}
                onChange={(v) => set("landHoldingAcres", v)}
              />
            )}
          </>
        )}

        {step === 3 && (
          <>
            <Select
              label="What do you need the money for?"
              value={profile.purpose}
              onChange={(v) => set("purpose", v as Purpose)}
              options={Object.entries(PURPOSES).map(([value, label]) => ({
                value,
                label,
              }))}
            />
            <Number
              label="How much do you need?"
              hint={rupeesShort(profile.amountNeededInr)}
              value={profile.amountNeededInr}
              step={10000}
              onChange={(v) => set("amountNeededInr", v)}
            />
            <Number
              label="Annual family income"
              hint={`${rupeesShort(profile.annualIncomeInr)} — several schemes have an income ceiling`}
              value={profile.annualIncomeInr}
              step={25000}
              onChange={(v) => set("annualIncomeInr", v)}
            />
          </>
        )}

        {step === 4 && (
          <>
            <p className="text-sm text-zinc-400">
              These rarely disqualify anyone. Anything missing becomes a step
              you can take to unlock more schemes.
            </p>
            <Toggle
              label="Aadhaar, linked to my mobile number"
              checked={profile.hasAadhaar}
              onChange={(v) => set("hasAadhaar", v)}
            />
            <Toggle
              label="A bank account in my name"
              checked={profile.hasBankAccount}
              onChange={(v) => set("hasBankAccount", v)}
            />
            <Toggle
              label="Udyam (MSME) registration"
              checked={profile.hasUdyam}
              onChange={(v) => set("hasUdyam", v)}
            />
            <Toggle
              label="A disability certificate (40% or more)"
              checked={profile.hasDisability}
              onChange={(v) => set("hasDisability", v)}
            />

            {runsBusiness && (
              <>
                <Number
                  label="How many months has your business been running?"
                  value={profile.businessVintageMonths}
                  onChange={(v) => set("businessVintageMonths", v)}
                />
                <Number
                  label="Annual turnover"
                  hint={rupeesShort(profile.annualTurnoverInr)}
                  value={profile.annualTurnoverInr}
                  step={50000}
                  onChange={(v) => set("annualTurnoverInr", v)}
                />
              </>
            )}
          </>
        )}

        {step === 5 && (
          <>
            <Select
              label="Which language should we search in?"
              hint="State portals publish in the local language, so this finds schemes an English search never returns."
              value={profile.language ?? "en"}
              onChange={(v) => set("language", v)}
              options={languages.map((l) => ({ value: l.code, label: l.label }))}
            />
            <div className="rounded-lg border border-zinc-800 bg-zinc-950/60 p-3 text-xs leading-relaxed text-zinc-500">
              Scheme Setu never asks for a fee and never applies on your
              behalf. Every link goes to an official government portal.
            </div>
          </>
        )}
      </div>

      <div className="mt-7 flex items-center gap-3">
        {step > 0 && (
          <button
            type="button"
            onClick={() => setStep((s) => s - 1)}
            className="rounded-lg border border-zinc-700 px-3.5 py-2 text-sm text-zinc-300 transition hover:bg-zinc-800"
          >
            Back
          </button>
        )}
        <button
          type="submit"
          disabled={loading}
          className="rounded-lg bg-accent-600 px-4 py-2 text-sm font-semibold text-zinc-950 transition hover:bg-accent-500 disabled:opacity-50"
        >
          {loading ? "Searching…" : last ? "Find my schemes" : "Next"}
        </button>
        {last && !loading && (
          <button
            type="button"
            onClick={() => {
              setProfile(INITIAL);
              setStep(0);
            }}
            className="text-xs text-zinc-500 underline underline-offset-2 hover:text-zinc-300"
          >
            Reset
          </button>
        )}
      </div>
    </form>
  );
}

/* --- small form primitives, kept local so there is no UI dependency --- */

const labelClass = "block text-sm font-medium text-zinc-300";
const hintClass = "mt-1 text-xs leading-relaxed text-zinc-500";
const inputClass =
  "mt-1.5 w-full rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-accent-500";

function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <label className="block">
      <span className={labelClass}>{label}</span>
      {children}
      {hint && <span className={hintClass}>{hint}</span>}
    </label>
  );
}

function Select({
  label,
  hint,
  value,
  onChange,
  options,
}: {
  label: string;
  hint?: string;
  value: string;
  onChange: (v: string) => void;
  options: { value: string; label: string }[];
}) {
  return (
    <Field label={label} hint={hint}>
      <select
        className={inputClass}
        value={value}
        onChange={(e) => onChange(e.target.value)}
      >
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </Field>
  );
}

function Text({
  label,
  hint,
  value,
  onChange,
}: {
  label: string;
  hint?: string;
  value: string;
  onChange: (v: string) => void;
}) {
  return (
    <Field label={label} hint={hint}>
      <input
        className={inputClass}
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
    </Field>
  );
}

function Number({
  label,
  hint,
  value,
  onChange,
  min = 0,
  max,
  step = 1,
}: {
  label: string;
  hint?: string;
  value: number;
  onChange: (v: number) => void;
  min?: number;
  max?: number;
  step?: number;
}) {
  return (
    <Field label={label} hint={hint}>
      <input
        type="number"
        className={`${inputClass} nums`}
        value={value}
        min={min}
        max={max}
        step={step}
        onChange={(e) => onChange(globalThis.Number(e.target.value) || 0)}
      />
    </Field>
  );
}

function Radio({
  label,
  hint,
  value,
  onChange,
  options,
}: {
  label: string;
  hint?: string;
  value: string;
  onChange: (v: string) => void;
  options: { value: string; label: string }[];
}) {
  return (
    <fieldset>
      <legend className={labelClass}>{label}</legend>
      <div className="mt-2 flex flex-wrap gap-2">
        {options.map((o) => (
          <button
            key={o.value}
            type="button"
            onClick={() => onChange(o.value)}
            aria-pressed={value === o.value}
            className={`rounded-lg border px-3 py-1.5 text-sm transition ${
              value === o.value
                ? "border-accent-500 bg-accent-500/10 text-accent-400"
                : "border-zinc-700 text-zinc-400 hover:bg-zinc-800"
            }`}
          >
            {o.label}
          </button>
        ))}
      </div>
      {hint && <p className={hintClass}>{hint}</p>}
    </fieldset>
  );
}

function Toggle({
  label,
  checked,
  onChange,
}: {
  label: string;
  checked: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <label className="flex cursor-pointer items-center gap-3">
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        className="size-4 accent-accent-500"
      />
      <span className="text-sm text-zinc-300">{label}</span>
    </label>
  );
}
