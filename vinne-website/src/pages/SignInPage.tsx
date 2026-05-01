import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Eye, EyeOff, Phone } from "lucide-react";
import Navbar from "@/components/Navbar";
import Footer from "@/components/Footer";
import PhoneInput from "@/components/PhoneInput";
import USSDLoginFlow from "@/components/USSDLoginFlow";
import ForgotPasswordFlow from "@/components/ForgotPasswordFlow";
import { toast } from "@/hooks/use-toast";
import { API_BASE } from "@/lib/config";

const ERROR_MESSAGES: Record<string, string> = {
  "invalid credentials": "Incorrect phone number or password. Please double-check and try again.",
  "unauthorized": "Incorrect phone number or password. Please double-check and try again.",
  "401": "Incorrect phone number or password. Please double-check and try again.",
  "user not found": "No account found with that phone number. Please check the number or sign up for a new account.",
  "account suspended": "Your account has been temporarily suspended. Please contact our support team for assistance.",
  "too many attempts": "Too many failed attempts. Please wait a few minutes before trying again.",
  "400": "Please check your phone number and password, then try again.",
  "500": "Our servers are having a moment. Please try again in a few seconds.",
  "network": "Connection issue. Please check your internet and try again.",
  "timeout": "Request timed out. Please try again.",
};

function friendlyError(raw: string): string {
  const lower = raw.toLowerCase();
  for (const [key, msg] of Object.entries(ERROR_MESSAGES)) {
    if (lower.includes(key)) return msg;
  }
  if (raw.includes('401') || raw.includes('unauthorized')) return "Incorrect phone number or password. Please double-check and try again.";
  if (raw.includes('400')) return "Please check your phone number and password, then try again.";
  if (raw.includes('500')) return "Our servers are having a moment. Please try again in a few seconds.";
  if (raw.includes('timeout') || raw.includes('network')) return "Connection issue. Please check your internet and try again.";
  return "Something went wrong. Please check your details and try again.";
}


const SignInPage = () => {
  const navigate = useNavigate();
  const [form, setForm] = useState({ phone: "", password: "" });
  const [showPwd, setShowPwd] = useState(false);
  const [loading, setLoading] = useState(false);
  const [showUSSD, setShowUSSD] = useState(false);
  const [showForgot, setShowForgot] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      const res = await fetch(`${API_BASE}/players/login`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ phone_number: form.phone, password: form.password, channel: "web" }),
      });
      const data = await res.json();
      if (!res.ok || data.error) {
        const msg = data.error || data.message || `Login failed (${res.status})`;
        throw new Error(msg);
      }
      if (data.requires_otp) {
        toast({ title: "OTP Required", description: "Please verify your phone number to continue." });
        return;
      }
      localStorage.setItem("player_token", data.access_token);
      if (data.profile?.id) localStorage.setItem("player_id", data.profile.id);
      toast({ title: "Welcome back! 🎉", description: "You are now signed in." });
      navigate("/");
    } catch (err: unknown) {
      toast({ title: "Sign in failed", description: friendlyError((err as Error).message), variant: "destructive" });
    } finally {
      setLoading(false);
    }
  };

  const inputCls = "w-full bg-secondary text-foreground placeholder:text-muted-foreground border border-border rounded-lg px-4 py-3 text-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary transition";

  return (
    <div className="min-h-screen flex flex-col bg-background">
      <Navbar />
      <main className="flex-1 flex items-center justify-center pt-24 pb-16 px-4">
        <div className="w-full max-w-md">
          <h1 className="font-heading text-3xl text-primary mb-1 text-center tracking-wide">SIGN IN</h1>
          <p className="text-muted-foreground text-center mb-8 text-sm">Welcome back — enter your details to continue</p>

          {showForgot ? (
            <ForgotPasswordFlow
              onSuccess={() => { setShowForgot(false); toast({ title: "Password updated!", description: "Sign in with your new password." }); }}
              onBack={() => setShowForgot(false)}
            />
          ) : !showUSSD ? (
            <>
              <form onSubmit={handleSubmit} className="bg-card rounded-2xl p-8 space-y-5 border border-border shadow-lg">
                <div className="space-y-1.5">
                  <label className="block text-sm font-medium text-foreground">Phone Number</label>
                  <PhoneInput value={form.phone} onChange={v => setForm(f => ({ ...f, phone: v }))} required placeholder="244 123 456" />
                </div>

                <div className="space-y-1.5">
                  <div className="flex items-center justify-between">
                    <label className="block text-sm font-medium text-foreground">Password</label>
                    <button type="button" onClick={() => setShowForgot(true)}
                      className="text-xs text-primary hover:underline">
                      Forgot password?
                    </button>
                  </div>
                  <div className="relative">
                    <input type={showPwd ? "text" : "password"} placeholder="Enter your password"
                      value={form.password} onChange={e => setForm(f => ({ ...f, password: e.target.value }))}
                      required className={`${inputCls} pr-11`} />
                    <button type="button" onClick={() => setShowPwd(v => !v)}
                      className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition">
                      {showPwd ? <EyeOff size={18} /> : <Eye size={18} />}
                    </button>
                  </div>
                </div>

                <button type="submit" disabled={loading}
                  className="w-full bg-primary text-white font-heading py-3 rounded-lg btn-glow hover:brightness-110 transition disabled:opacity-60 tracking-wide text-base">
                  {loading ? "Signing in..." : "SIGN IN"}
                </button>

                <p className="text-center text-sm text-muted-foreground">
                  Don't have an account?{" "}
                  <Link to="/sign-up" className="text-primary font-semibold hover:underline">Sign Up</Link>
                </p>
              </form>

              {/* USSD ticket holder entry point */}
              <button onClick={() => setShowUSSD(true)}
                className="mt-4 w-full flex items-center justify-center gap-2 text-sm text-muted-foreground border border-border rounded-xl px-4 py-3 hover:border-primary/50 hover:text-primary transition bg-card">
                <Phone size={15} />
                Already bought tickets? Sign in with your number
              </button>
            </>
          ) : (
            <USSDLoginFlow onSuccess={() => navigate("/")} onBack={() => setShowUSSD(false)} />
          )}
        </div>
      </main>
      <Footer />
    </div>
  );
};

export default SignInPage;
