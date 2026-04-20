import { useState, useRef, useEffect } from "react";
import { ChevronDown } from "lucide-react";

const COUNTRIES = [
  { code: "GH", name: "Ghana",        dial: "+233", flag: "🇬🇭" },
  { code: "NG", name: "Nigeria",       dial: "+234", flag: "🇳🇬" },
  { code: "KE", name: "Kenya",         dial: "+254", flag: "🇰🇪" },
  { code: "ZA", name: "South Africa",  dial: "+27",  flag: "🇿🇦" },
  { code: "UG", name: "Uganda",        dial: "+256", flag: "🇺🇬" },
  { code: "TZ", name: "Tanzania",      dial: "+255", flag: "🇹🇿" },
  { code: "CI", name: "Côte d'Ivoire", dial: "+225", flag: "🇨🇮" },
  { code: "SN", name: "Senegal",       dial: "+221", flag: "🇸🇳" },
  { code: "CM", name: "Cameroon",      dial: "+237", flag: "🇨🇲" },
  { code: "RW", name: "Rwanda",        dial: "+250", flag: "🇷🇼" },
  { code: "ZM", name: "Zambia",        dial: "+260", flag: "🇿🇲" },
  { code: "ZW", name: "Zimbabwe",      dial: "+263", flag: "🇿🇼" },
  { code: "GB", name: "UK",            dial: "+44",  flag: "🇬🇧" },
  { code: "US", name: "USA",           dial: "+1",   flag: "🇺🇸" },
];

interface Props {
  value: string;
  onChange: (fullNumber: string) => void;
  required?: boolean;
  placeholder?: string;
}

const PhoneInput = ({ value, onChange, required, placeholder }: Props) => {
  const [country, setCountry] = useState(COUNTRIES[0]); // Ghana default
  const [local, setLocal] = useState("");
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const ref = useRef<HTMLDivElement>(null);

  // Close on outside click
  useEffect(() => {
    const h = (e: MouseEvent) => { if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false); };
    document.addEventListener("mousedown", h);
    return () => document.removeEventListener("mousedown", h);
  }, []);

  // Sync value prop back to local state (for controlled usage)
  useEffect(() => {
    if (!value) { setLocal(""); return; }
    // Strip dial code if present
    const match = COUNTRIES.find(c => value.startsWith(c.dial));
    if (match) { setCountry(match); setLocal(value.slice(match.dial.length)); }
    else setLocal(value);
  }, []); // only on mount

  const handleLocal = (e: React.ChangeEvent<HTMLInputElement>) => {
    // Strip leading zeros for international format
    const raw = e.target.value.replace(/[^\d]/g, "");
    setLocal(raw);
    const stripped = raw.startsWith("0") ? raw.slice(1) : raw;
    onChange(`${country.dial}${stripped}`);
  };

  const selectCountry = (c: typeof COUNTRIES[0]) => {
    setCountry(c);
    const stripped = local.startsWith("0") ? local.slice(1) : local;
    onChange(`${c.dial}${stripped}`);
    setOpen(false);
    setSearch("");
  };

  const filtered = COUNTRIES.filter(c =>
    c.name.toLowerCase().includes(search.toLowerCase()) ||
    c.dial.includes(search)
  );

  const inputCls = "flex-1 bg-secondary text-foreground placeholder:text-muted-foreground border-0 rounded-r-lg px-3 py-3 text-sm focus:outline-none focus:ring-0 transition min-w-0";

  return (
    <div className="relative flex w-full bg-secondary border border-border rounded-lg focus-within:ring-2 focus-within:ring-primary focus-within:border-primary transition" ref={ref}>
      {/* Country selector */}
      <button type="button" onClick={() => setOpen(v => !v)}
        className="flex items-center gap-1.5 px-3 py-3 border-r border-border text-sm shrink-0 hover:bg-border/50 rounded-l-lg transition">
        <span className="text-base leading-none">{country.flag}</span>
        <span className="text-foreground font-medium">{country.dial}</span>
        <ChevronDown size={13} className={`text-muted-foreground transition-transform ${open ? "rotate-180" : ""}`} />
      </button>

      {/* Number input */}
      <input
        type="tel"
        value={local}
        onChange={handleLocal}
        placeholder={placeholder || "244 123 456"}
        required={required}
        className={inputCls}
      />

      {/* Dropdown */}
      {open && (
        <div className="absolute top-full left-0 mt-1 w-64 bg-card border border-border rounded-xl shadow-2xl z-50 overflow-hidden">
          <div className="p-2 border-b border-border">
            <input
              type="text"
              placeholder="Search country..."
              value={search}
              onChange={e => setSearch(e.target.value)}
              className="w-full bg-secondary text-foreground placeholder:text-muted-foreground text-sm px-3 py-2 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary"
              autoFocus
            />
          </div>
          <div className="max-h-52 overflow-y-auto">
            {filtered.map(c => (
              <button key={c.code} type="button" onClick={() => selectCountry(c)}
                className={`w-full flex items-center gap-3 px-4 py-2.5 text-sm hover:bg-secondary transition text-left ${c.code === country.code ? "bg-primary/10 text-primary" : "text-foreground"}`}>
                <span className="text-base">{c.flag}</span>
                <span className="flex-1">{c.name}</span>
                <span className="text-muted-foreground text-xs">{c.dial}</span>
              </button>
            ))}
            {filtered.length === 0 && <p className="text-center text-muted-foreground text-sm py-4">No results</p>}
          </div>
        </div>
      )}
    </div>
  );
};

export default PhoneInput;
