import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Eye, EyeOff } from "lucide-react";
import Navbar from "@/components/Navbar";
import Footer from "@/components/Footer";
import { toast } from "@/hooks/use-toast";

const ERROR_MESSAGES: Record<string, string> = {
  "already exists": "An account with this phone number already exists. Try signing in instead.",
  "phone number": "Please enter a valid Ghana phone number (e.g. 0244123456).",
  "password": "Password must be at least 6 characters long.",
  "invalid": "Some details look incorrect. Please check and try again.",
};

function friendlyError(raw: string): string {
  const lower = raw.toLowerCase();
  for (const [key, msg] of Object.entries(ERROR_MESSAGES)) {
    if (lower.includes(key)) return msg;
  }
  return "Something went wrong. Please check your details and try again.";
}

const SignUpPage = () => {
  const navigate = useNavigate();
  const [form, setForm] = useState({ phone: "", password: "", confirm: "" });
  const [showPwd, setShowPwd] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);
  const [loading, setLoading] = useState(false);

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
      const res = await fetch("/api/v1/players/register", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ phone_number: form.phone, password: form.password, channel: "web", terms_accepted: true }),
      });
      const data = await res.json();
      if (!res.ok || data.error) throw new Error(data.error || data.message || "Registration failed");
      if (data.requires_otp) {
        toast({ title: "Verify your number", description: "An OTP has been sent to your phone." });
        return;
      }
      if (data.access_token) localStorage.setItem("player_token", data.access_token);
      if (data.profile?.id) localStorage.setItem("player_id", data.profile.id);
      toast({ title: "Account created! 🎉", description: "Welcome to WinBig Africa." });
      navigate("/");
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

          <form onSubmit={handleSubmit} className="bg-card rounded-2xl p-8 space-y-5 border border-border shadow-lg">
            <div className="space-y-1.5">
              <label className="block text-sm font-medium text-foreground">Phone Number</label>
              <input type="tel" placeholder="e.g. 0244123456" value={form.phone}
                onChange={e => setForm(f => ({ ...f, phone: e.target.value }))} required className={inputCls} />
            </div>

            <div className="space-y-1.5">
              <label className="block text-sm font-medium text-foreground">Password</label>
              <div className="relative">
                <input
                  type={showPwd ? "text" : "password"}
                  placeholder="At least 6 characters"
                  value={form.password}
                  onChange={e => setForm(f => ({ ...f, password: e.target.value }))}
                  required
                  className={`${inputCls} pr-11`}
                />
                <button type="button" onClick={() => setShowPwd(v => !v)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition">
                  {showPwd ? <EyeOff size={18} /> : <Eye size={18} />}
                </button>
              </div>
            </div>

            <div className="space-y-1.5">
              <label className="block text-sm font-medium text-foreground">Confirm Password</label>
              <div className="relative">
                <input
                  type={showConfirm ? "text" : "password"}
                  placeholder="Repeat your password"
                  value={form.confirm}
                  onChange={e => setForm(f => ({ ...f, confirm: e.target.value }))}
                  required
                  className={`${inputCls} pr-11`}
                />
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
          </form>
        </div>
      </main>
      <Footer />
    </div>
  );
};

export default SignUpPage;
