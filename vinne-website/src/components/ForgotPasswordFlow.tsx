import { useState } from "react";
import { ArrowLeft, Eye, EyeOff, Loader2 } from "lucide-react";
import PhoneInput from "@/components/PhoneInput";
import { toast } from "@/hooks/use-toast";
import { API_BASE } from "@/lib/config";

type Step = "phone" | "otp" | "password";

interface Props {
  onSuccess: () => void;
  onBack: () => void;
}

const ForgotPasswordFlow = ({ onSuccess, onBack }: Props) => {
  const [step, setStep] = useState<Step>("phone");
  const [phone, setPhone] = useState("");
  const [code, setCode] = useState("");
  const [newPwd, setNewPwd] = useState("");
  const [confirmPwd, setConfirmPwd] = useState("");
  const [showPwd, setShowPwd] = useState(false);
  const [loading, setLoading] = useState(false);

  const inputCls =
    "w-full bg-secondary text-foreground placeholder:text-muted-foreground border border-border rounded-lg px-4 py-3 text-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary transition";

  const sendCode = async () => {
    if (!phone) return;
    setLoading(true);
    try {
      const res = await fetch(`${API_BASE}/players/forgot-password`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ phone_number: phone }),
      });
      const d = await res.json();
      if (!res.ok && d.error) throw new Error(d.error);
      toast({ title: "Code sent", description: "Check your phone for the reset code." });
      setStep("otp");
    } catch (err: unknown) {
      toast({ title: "Error", description: (err as Error).message || "Something went wrong.", variant: "destructive" });
    } finally {
      setLoading(false);
    }
  };

  const verifyCode = () => {
    if (code.length < 4) return;
    setStep("password");
  };

  const resetPassword = async () => {
    if (newPwd.length < 6) {
      toast({ title: "Too short", description: "Password must be at least 6 characters.", variant: "destructive" });
      return;
    }
    if (newPwd !== confirmPwd) {
      toast({ title: "Mismatch", description: "Passwords do not match.", variant: "destructive" });
      return;
    }
    setLoading(true);
    try {
      const res = await fetch(`${API_BASE}/players/reset-password`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ phone_number: phone, code, new_password: newPwd }),
      });
      const d = await res.json();
      if (!res.ok || d.error) throw new Error(d.error || "Reset failed");
      toast({ title: "Password updated!", description: "You can now sign in with your new password." });
      onSuccess();
    } catch (err: unknown) {
      const msg = (err as Error).message || "";
      if (msg.toLowerCase().includes("different password") || msg.toLowerCase().includes("same")) {
        toast({ title: "Same password", description: "Please choose a different password from your current one.", variant: "destructive" });
      } else if (msg.toLowerCase().includes("expired") || msg.toLowerCase().includes("not found")) {
        toast({ title: "Code expired", description: "Your reset code expired. Please start over.", variant: "destructive" });
        setStep("phone");
        setCode("");
      } else if (msg.toLowerCase().includes("invalid")) {
        toast({ title: "Invalid code", description: "Wrong reset code. Please check and try again.", variant: "destructive" });
      } else {
        toast({ title: "Reset failed", description: msg || "Please try again.", variant: "destructive" });
      }
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
          <h2 className="font-heading text-lg text-foreground">Reset your password</h2>
          <p className="text-xs text-muted-foreground">
            {step === "phone" && "Enter your registered phone number"}
            {step === "otp" && "Enter the code we sent to your phone"}
            {step === "password" && "Choose a new password"}
          </p>
        </div>
      </div>

      {step === "phone" && (
        <div className="space-y-4">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-foreground">Phone Number</label>
            <PhoneInput value={phone} onChange={setPhone} required placeholder="244 123 456" />
          </div>
          <button
            onClick={sendCode}
            disabled={loading || !phone}
            className="w-full bg-primary text-white font-heading py-3 rounded-lg btn-glow hover:brightness-110 transition disabled:opacity-60 tracking-wide text-base flex items-center justify-center gap-2"
          >
            {loading ? <Loader2 size={16} className="animate-spin" /> : null}
            {loading ? "Sending..." : "Send Reset Code"}
          </button>
        </div>
      )}

      {step === "otp" && (
        <div className="space-y-4">
          <p className="text-sm text-muted-foreground">
            Enter the 6-digit code sent to <span className="text-foreground font-medium">{phone}</span>
          </p>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-foreground">Reset Code</label>
            <input
              value={code}
              onChange={e => setCode(e.target.value)}
              placeholder="000000"
              maxLength={6}
              className={inputCls}
              autoFocus
            />
          </div>
          <button
            onClick={verifyCode}
            disabled={code.length < 4}
            className="w-full bg-primary text-white font-heading py-3 rounded-lg btn-glow hover:brightness-110 transition disabled:opacity-60 tracking-wide text-base"
          >
            Continue
          </button>
          <button
            onClick={() => { setStep("phone"); setCode(""); }}
            className="w-full flex items-center justify-center gap-1.5 text-sm text-muted-foreground hover:text-primary transition"
          >
            <ArrowLeft size={14} /> Use a different number
          </button>
        </div>
      )}

      {step === "password" && (
        <div className="space-y-4">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-foreground">New Password</label>
            <div className="relative">
              <input
                type={showPwd ? "text" : "password"}
                placeholder="At least 6 characters"
                value={newPwd}
                onChange={e => setNewPwd(e.target.value)}
                className={`${inputCls} pr-11`}
                autoFocus
              />
              <button
                type="button"
                onClick={() => setShowPwd(v => !v)}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition"
              >
                {showPwd ? <EyeOff size={18} /> : <Eye size={18} />}
              </button>
            </div>
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-foreground">Confirm New Password</label>
            <input
              type="password"
              placeholder="Repeat your new password"
              value={confirmPwd}
              onChange={e => setConfirmPwd(e.target.value)}
              className={inputCls}
            />
          </div>
          <button
            onClick={resetPassword}
            disabled={loading || newPwd.length < 6 || confirmPwd.length < 1}
            className="w-full bg-primary text-white font-heading py-3 rounded-lg btn-glow hover:brightness-110 transition disabled:opacity-60 tracking-wide text-base flex items-center justify-center gap-2"
          >
            {loading ? <Loader2 size={16} className="animate-spin" /> : null}
            {loading ? "Updating..." : "Set New Password"}
          </button>
        </div>
      )}
    </div>
  );
};

export default ForgotPasswordFlow;
