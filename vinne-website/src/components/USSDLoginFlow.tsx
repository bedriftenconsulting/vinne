import { useState } from "react";
import { Phone, ArrowLeft, Loader2 } from "lucide-react";
import PhoneInput from "@/components/PhoneInput";
import { toast } from "@/hooks/use-toast";
import { API_BASE } from "@/lib/config";

type Step = "phone" | "otp";

function friendlyError(raw: string): string {
  const lower = raw.toLowerCase();
  if (lower.includes("no tickets") || lower.includes("not found")) return "No tickets found for this number. Please check the number or sign up.";
  if (lower.includes("invalid") || lower.includes("expired")) return "Invalid or expired code. Please try again.";
  return "Something went wrong. Please try again.";
}

interface Props {
  onSuccess: () => void;
  onBack: () => void;
}

const USSDLoginFlow = ({ onSuccess, onBack }: Props) => {
  const [step, setStep] = useState<Step>("phone");
  const [phone, setPhone] = useState("");
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [loading, setLoading] = useState(false);

  const inputCls = "w-full bg-secondary text-foreground placeholder:text-muted-foreground border border-border rounded-lg px-4 py-3 text-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary transition";

  const requestOTP = async () => {
    if (!phone) return;
    setLoading(true);
    try {
      const res = await fetch(`${API_BASE}/players/ussd-otp/request`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ phone_number: phone }),
      });
      const d = await res.json();
      if (!res.ok || d.error) throw new Error(d.error || "Failed to send OTP");
      toast({ title: "Code sent", description: `Check your phone (${phone}) for the code.` });
      setStep("otp");
    } catch (err: unknown) {
      toast({ title: "Not found", description: friendlyError((err as Error).message), variant: "destructive" });
    } finally {
      setLoading(false);
    }
  };

  const verifyOTP = async () => {
    if (!code) return;
    setLoading(true);
    try {
      const res = await fetch(`${API_BASE}/players/ussd-otp/verify`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ phone_number: phone, code }),
      });
      const d = await res.json();
      if (!res.ok || d.error) throw new Error(d.error || "Verification failed");

      const token = d.access_token;
      const playerId = d.profile?.id;
      localStorage.setItem("player_token", token);
      if (playerId) localStorage.setItem("player_id", playerId);
      window.dispatchEvent(new Event("storage"));

      // Update name if provided and player ID is available
      if (name.trim() && playerId) {
        const parts = name.trim().split(" ");
        const first_name = parts[0] || "";
        const last_name = parts.slice(1).join(" ") || "";
        try {
          await fetch(`${API_BASE}/players/${playerId}/profile`, {
            method: "PUT",
            headers: { "Content-Type": "application/json", "Authorization": `Bearer ${token}` },
            body: JSON.stringify({ first_name, last_name }),
          });
        } catch {
          // Name update failure is non-fatal — user is still logged in
        }
      }

      toast({ title: "Welcome!", description: "You are now signed in." });
      onSuccess();
    } catch (err: unknown) {
      toast({ title: "Verification failed", description: friendlyError((err as Error).message), variant: "destructive" });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="bg-card rounded-2xl p-8 border border-border shadow-lg space-y-5">
      <div className="flex items-center gap-3 mb-1">
        <button onClick={onBack} className="text-muted-foreground hover:text-primary transition">
          <ArrowLeft size={18} />
        </button>
        <div>
          <h2 className="font-heading text-lg text-foreground">Bought tickets via USSD?</h2>
          <p className="text-xs text-muted-foreground">
            {step === "phone" ? "Enter your details to access your tickets" : "We'll send a login code to your phone"}
          </p>
        </div>
      </div>

      {step === "phone" ? (
        <div className="space-y-4">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-foreground">
              Full Name <span className="text-muted-foreground font-normal">(optional)</span>
            </label>
            <input
              type="text"
              placeholder="e.g. Kwame Mensah"
              value={name}
              onChange={e => setName(e.target.value)}
              className={inputCls}
            />
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-foreground">Your Phone Number</label>
            <PhoneInput value={phone} onChange={setPhone} required placeholder="244 123 456" />
            <p className="text-xs text-muted-foreground">The number you used to buy tickets via USSD (*899*92#)</p>
          </div>
          <button onClick={requestOTP} disabled={loading || !phone}
            className="w-full bg-primary text-white font-heading py-3 rounded-lg btn-glow hover:brightness-110 transition disabled:opacity-60 tracking-wide text-base flex items-center justify-center gap-2">
            {loading ? <Loader2 size={16} className="animate-spin" /> : <Phone size={16} />}
            {loading ? "Checking..." : "Send Login Code"}
          </button>
        </div>
      ) : (
        <div className="space-y-4">
          <p className="text-sm text-muted-foreground">Enter the 6-digit code sent to <span className="text-foreground font-medium">{phone}</span></p>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-foreground">Verification Code</label>
            <input value={code} onChange={e => setCode(e.target.value)} placeholder="000000" maxLength={6}
              className={inputCls} autoFocus />
          </div>
          <button onClick={verifyOTP} disabled={loading || code.length < 4}
            className="w-full bg-primary text-white font-heading py-3 rounded-lg btn-glow hover:brightness-110 transition disabled:opacity-60 tracking-wide text-base flex items-center justify-center gap-2">
            {loading ? <Loader2 size={16} className="animate-spin" /> : null}
            {loading ? "Verifying..." : "Sign In"}
          </button>
          <button onClick={() => { setStep("phone"); setCode(""); }}
            className="w-full flex items-center justify-center gap-1.5 text-sm text-muted-foreground hover:text-primary transition">
            <ArrowLeft size={14} /> Use a different number
          </button>
        </div>
      )}
    </div>
  );
};

export default USSDLoginFlow;
