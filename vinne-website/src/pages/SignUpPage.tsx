import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Eye, EyeOff, Phone } from "lucide-react";
import Navbar from "@/components/Navbar";
import Footer from "@/components/Footer";
import PhoneInput from "@/components/PhoneInput";
import USSDLoginFlow from "@/components/USSDLoginFlow";
import { toast } from "@/hooks/use-toast";
import { API_BASE } from "@/lib/config";

const ERROR_MESSAGES: Record<string, string> = {
  "already exists": "This phone number is already registered with a password. Please sign in instead.",
  "phone number already registered": "This phone number is already registered with a password. Please sign in instead.",
  "phone number": "Please enter a valid Ghana phone number (e.g. 0244123456).",
  "password": "Password must be at least 6 characters long.",
  "invalid": "Some details look incorrect. Please check and try again.",
  "409": "This phone number already exists. Please try signing in instead, or use a different phone number.",
  "conflict": "This phone number already exists. Please try signing in instead, or use a different phone number.",
  "400": "Please check your details and try again. Make sure all fields are filled correctly.",
  "500": "Our servers are having a moment. Please try again in a few seconds.",
  "network": "Connection issue. Please check your internet and try again.",
  "timeout": "Request timed out. Please try again.",
};

function friendlyError(raw: string): string {
  const lower = raw.toLowerCase();
  for (const [key, msg] of Object.entries(ERROR_MESSAGES)) {
    if (lower.includes(key)) return msg;
  }
  
  // Handle specific HTTP status codes
  if (raw.includes('409')) {
    return "This phone number already exists. Please try signing in instead, or use a different phone number.";
  }
  if (raw.includes('400')) {
    return "Please check your details and try again. Make sure all fields are filled correctly.";
  }
  if (raw.includes('500')) {
    return "Our servers are having a moment. Please try again in a few seconds.";
  }
  if (raw.includes('timeout') || raw.includes('network')) {
    return "Connection issue. Please check your internet and try again.";
  }
  
  return "Something went wrong. Please check your details and try again, or contact support if the issue persists.";
}

const SignUpPage = () => {
  const navigate = useNavigate();
  const [form, setForm] = useState({
    full_name: "", email: "", phone: "", password: "", confirm: "",
  });  const [showPwd, setShowPwd] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);
  const [loading, setLoading] = useState(false);
  const [showUSSD, setShowUSSD] = useState(false);

  const set = (k: string) => (e: React.ChangeEvent<HTMLInputElement>) =>
    setForm(f => ({ ...f, [k]: e.target.value }));

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (form.password.length < 6) {
      toast({ title: "Password too short", description: "Password must be at least 6 characters.", variant: "destructive" });
      return;
    }
    if (form.password !== form.confirm) {
      toast({ title: "Passwords don't match", description: "Make sure both password fields are the same.", variant: "destructive" });
      return;
    }
    setLoading(true);
    try {
      // Split full name into first/last
      const parts = form.full_name.trim().split(" ");
      const first_name = parts[0] || "";
      const last_name = parts.slice(1).join(" ") || "";

      const res = await fetch(`${API_BASE}/players/register`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          phone_number: form.phone,
          password: form.password,
          channel: "web",
          terms_accepted: true,
          first_name,
          last_name,
          email: form.email,
        }),
      });
      const data = await res.json();
      if (!res.ok || data.error) throw new Error(data.error || data.message || "Registration failed");
      if (data.requires_otp) {
        toast({ title: "Verify your number", description: "An OTP has been sent to your phone." });
        return;
      }

      // If we got a token directly, store it and go home
      if (data.access_token) {
        localStorage.setItem("player_token", data.access_token);
        if (data.profile?.id) localStorage.setItem("player_id", data.profile.id);
        toast({ title: "Account created! 🎉", description: "Welcome to WinBig Africa." });
        window.dispatchEvent(new Event("storage"));
        navigate("/");
        return;
      }

      // Otherwise auto-login with the credentials they just used
      const loginRes = await fetch(`${API_BASE}/players/login`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ phone_number: form.phone, password: form.password, channel: "web" }),
      });
      const loginData = await loginRes.json();
      if (loginData.access_token) {
        localStorage.setItem("player_token", loginData.access_token);
        if (loginData.profile?.id) localStorage.setItem("player_id", loginData.profile.id);
        toast({ title: "Account created! 🎉", description: "Welcome to WinBig Africa." });
        window.dispatchEvent(new Event("storage"));
        navigate("/");
      } else {
        // Registration worked but auto-login failed — send to sign-in
        toast({ title: "Account created!", description: "Please sign in to continue." });
        navigate("/sign-in");
      }
    } catch (err: unknown) {
      toast({ title: "Sign up failed", description: friendlyError((err as Error).message), variant: "destructive" });
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
          <h1 className="font-heading text-3xl text-primary mb-1 text-center tracking-wide">CREATE ACCOUNT</h1>
          <p className="text-muted-foreground text-center mb-8 text-sm">Join WinBig Africa and start winning today</p>

          {showUSSD ? (
            <USSDLoginFlow onSuccess={() => navigate("/")} onBack={() => setShowUSSD(false)} />
          ) : null}

          {!showUSSD && <form onSubmit={handleSubmit} className="bg-card rounded-2xl p-8 space-y-5 border border-border shadow-lg">

            <div className="space-y-1.5">
              <label className="block text-sm font-medium text-foreground">Full Name</label>
              <input type="text" placeholder="e.g. Kwame Mensah" value={form.full_name}
                onChange={set("full_name")} required className={inputCls} />
            </div>

            <div className="space-y-1.5">
              <label className="block text-sm font-medium text-foreground">
                Email Address <span className="text-muted-foreground font-normal">(optional)</span>
              </label>
              <input type="email" placeholder="e.g. kwame@gmail.com" value={form.email}
                onChange={set("email")} className={inputCls} />
            </div>

            <div className="space-y-1.5">
              <label className="block text-sm font-medium text-foreground">Phone Number</label>
              <PhoneInput
                value={form.phone}
                onChange={v => setForm(f => ({ ...f, phone: v }))}
                required
                placeholder="244 123 456"
              />
            </div>

            <div className="space-y-1.5">
              <label className="block text-sm font-medium text-foreground">Password</label>
              <div className="relative">
                <input type={showPwd ? "text" : "password"} placeholder="At least 6 characters"
                  value={form.password} onChange={set("password")} required className={`${inputCls} pr-11`} />
                <button type="button" onClick={() => setShowPwd(v => !v)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition">
                  {showPwd ? <EyeOff size={18} /> : <Eye size={18} />}
                </button>
              </div>
            </div>

            <div className="space-y-1.5">
              <label className="block text-sm font-medium text-foreground">Confirm Password</label>
              <div className="relative">
                <input type={showConfirm ? "text" : "password"} placeholder="Repeat your password"
                  value={form.confirm} onChange={set("confirm")} required className={`${inputCls} pr-11`} />
                <button type="button" onClick={() => setShowConfirm(v => !v)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition">
                  {showConfirm ? <EyeOff size={18} /> : <Eye size={18} />}
                </button>
              </div>
            </div>

            <button type="submit" disabled={loading}
              className="w-full bg-primary text-white font-heading py-3 rounded-lg btn-glow hover:brightness-110 transition disabled:opacity-60 tracking-wide text-base">
              {loading ? "Creating account..." : "CREATE ACCOUNT"}
            </button>

            <p className="text-center text-sm text-muted-foreground">
              Already have an account?{" "}
              <Link to="/sign-in" className="text-primary font-semibold hover:underline">Sign In</Link>
            </p>
          </form>}

          {!showUSSD && (
            <button onClick={() => setShowUSSD(true)}
              className="mt-4 w-full flex items-center justify-center gap-2 text-sm text-muted-foreground border border-border rounded-xl px-4 py-3 hover:border-primary/50 hover:text-primary transition bg-card">
              <Phone size={15} />
              Already bought tickets? Sign in with your number
            </button>
          )}
        </div>
      </main>
      <Footer />
    </div>
  );
};

export default SignUpPage;
