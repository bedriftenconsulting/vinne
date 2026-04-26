import { useEffect, useState } from "react";
import { useNavigate, Link } from "react-router-dom";
import { User, Phone, Mail, Calendar, Shield, Ticket, LogOut, Edit2, Check, X, Loader2, CheckCircle, AlertCircle, Send } from "lucide-react";
import Navbar from "@/components/Navbar";
import Footer from "@/components/Footer";
import { toast } from "@/hooks/use-toast";
import { API_BASE } from "@/lib/config";

const OTP_BASE = `${API_BASE}/otp`;

const getToken = () => localStorage.getItem("player_token");
const getPlayerIdFromToken = (t: string | null): string | null => {
  if (!t) return null;
  try { return JSON.parse(atob(t.split(".")[1])).user_id || null; } catch { return null; }
};

interface Profile {
  id: string;
  phone_number: string;
  email: string;
  first_name: string;
  last_name: string;
  national_id?: string;
  date_of_birth?: string;
  kyc_status?: string;
  created_at?: string;
  phone_verified?: boolean;
  email_verified?: boolean;
}

const ProfilePage = () => {
  const navigate = useNavigate();
  const token = getToken();
  const playerId = getPlayerIdFromToken(token) || localStorage.getItem("player_id");

  const [profile, setProfile] = useState<Profile | null>(null);
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState({ first_name: "", last_name: "", email: "" });

  // Verification state
  const [phoneOtpSent, setPhoneOtpSent] = useState(false);
  const [emailOtpSent, setEmailOtpSent] = useState(false);
  const [phoneOtp, setPhoneOtp] = useState("");
  const [emailOtp, setEmailOtp] = useState("");
  const [verifyingPhone, setVerifyingPhone] = useState(false);
  const [verifyingEmail, setVerifyingEmail] = useState(false);
  const [phoneVerified, setPhoneVerified] = useState(false);
  const [emailVerified, setEmailVerified] = useState(false);

  // Rate limiting — max 3 OTP requests per hour
  const getRateLimit = (key: string) => {
    try {
      const data = JSON.parse(localStorage.getItem(key) || '{"count":0,"reset":0}')
      if (Date.now() > data.reset) return { count: 0, reset: Date.now() + 3600000 }
      return data
    } catch { return { count: 0, reset: Date.now() + 3600000 } }
  }
  const bumpRateLimit = (key: string) => {
    const data = getRateLimit(key)
    localStorage.setItem(key, JSON.stringify({ count: data.count + 1, reset: data.reset }))
  }
  const isRateLimited = (key: string) => getRateLimit(key).count >= 3

  useEffect(() => {
    if (!token || !playerId) { navigate("/sign-in"); return; }
    fetch(`${API_BASE}/players/${playerId}/profile`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then(r => r.json())
      .then(d => {
        const p = d?.data?.profile ?? d?.data ?? d?.profile ?? d;
        setProfile(p);
        setForm({ first_name: p.first_name || "", last_name: p.last_name || "", email: p.email || "" });
        // Check verification status from OTP service
        fetch(`${OTP_BASE}/status/${playerId}`)
          .then(r => r.json())
          .then(v => {
            setPhoneVerified(!!v.phone_verified || !!p.phone_verified || p.kyc_status === "VERIFIED")
            setEmailVerified(!!v.email_verified || !!p.email_verified)
          })
          .catch(() => {
            setPhoneVerified(!!p.phone_verified || p.kyc_status === "VERIFIED")
            setEmailVerified(!!p.email_verified)
          })
      })
      .catch(() => toast({ title: "Failed to load profile", variant: "destructive" }))
      .finally(() => setLoading(false));
  }, [navigate, token, playerId]);

  const handleSave = async () => {
    if (!playerId) return;
    setSaving(true);
    try {
      const res = await fetch(`${API_BASE}/players/${playerId}/profile`, {
        method: "PUT",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify(form),
      });
      const d = await res.json();
      if (!res.ok || d.error) throw new Error(d.error || "Update failed");
      const p = d?.data?.profile ?? d?.data ?? d?.profile ?? d;
      setProfile(prev => ({ ...prev!, ...p, ...form }));
      setEditing(false);
      toast({ title: "Profile updated ✓" });
    } catch (err: unknown) {
      toast({ title: "Update failed", description: (err as Error).message, variant: "destructive" });
    } finally {
      setSaving(false);
    }
  };

  const sendPhoneOtp = async () => {
    if (isRateLimited('otp_phone')) {
      toast({ title: "Too many attempts", description: "Please wait 1 hour before requesting another code.", variant: "destructive" }); return;
    }
    bumpRateLimit('otp_phone')
    try {
      const res = await fetch(`${OTP_BASE}/send`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ player_id: playerId, channel: "phone", contact: profile?.phone_number }),
      })
      const d = await res.json()
      if (!res.ok || d.error) throw new Error(d.error || "Failed to send OTP")
      setPhoneOtpSent(true)
      toast({ title: "OTP sent", description: `A verification code was sent to ${profile?.phone_number}` })
    } catch (e: unknown) {
      toast({ title: "Failed to send OTP", description: (e as Error).message, variant: "destructive" })
    }
  }

  const sendEmailOtp = async () => {
    if (!profile?.email) { toast({ title: "Add your email first", variant: "destructive" }); return; }
    if (isRateLimited('otp_email')) {
      toast({ title: "Too many attempts", description: "Please wait 1 hour before requesting another code.", variant: "destructive" }); return;
    }
    bumpRateLimit('otp_email')
    try {
      const res = await fetch(`${OTP_BASE}/send`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ player_id: playerId, channel: "email", contact: profile.email }),
      })
      const d = await res.json()
      if (!res.ok || d.error) throw new Error(d.error || "Failed to send")
      setEmailOtpSent(true)
      toast({ title: "Verification email sent", description: `Check ${profile.email} for your code` })
    } catch (e: unknown) {
      toast({ title: "Failed to send", description: (e as Error).message, variant: "destructive" })
    }
  }

  const verifyPhoneOtp = async () => {
    if (!phoneOtp || phoneOtp.length < 4) { toast({ title: "Enter the OTP code", variant: "destructive" }); return; }
    setVerifyingPhone(true)
    try {
      const res = await fetch(`${OTP_BASE}/verify`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ player_id: playerId, channel: "phone", code: phoneOtp }),
      })
      const d = await res.json()
      if (!res.ok || d.error) throw new Error(d.error || "Verification failed")
      setPhoneVerified(true); setPhoneOtpSent(false); setPhoneOtp("")
      toast({ title: "Phone verified ✓", description: "Your phone number is now verified." })
    } catch (e: unknown) {
      toast({ title: "Verification failed", description: (e as Error).message, variant: "destructive" })
    } finally { setVerifyingPhone(false) }
  }

  const verifyEmailOtp = async () => {
    if (!emailOtp || emailOtp.length < 4) { toast({ title: "Enter the OTP code", variant: "destructive" }); return; }
    setVerifyingEmail(true)
    try {
      const res = await fetch(`${OTP_BASE}/verify`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ player_id: playerId, channel: "email", code: emailOtp }),
      })
      const d = await res.json()
      if (!res.ok || d.error) throw new Error(d.error || "Verification failed")
      setEmailVerified(true); setEmailOtpSent(false); setEmailOtp("")
      toast({ title: "Email verified ✓", description: "Your email address is now verified." })
    } catch (e: unknown) {
      toast({ title: "Verification failed", description: (e as Error).message, variant: "destructive" })
    } finally { setVerifyingEmail(false) }
  }

  const signOut = () => {
    localStorage.removeItem("player_token");
    localStorage.removeItem("player_id");
    navigate("/");
    window.dispatchEvent(new Event("storage"));
  };

  const fullName = profile
    ? [profile.first_name, profile.last_name].filter(Boolean).join(" ") || "Player"
    : "Player";

  const initials = fullName.split(" ").map(w => w[0]).join("").toUpperCase().slice(0, 2) || "P";

  const fmtDate = (d?: string) => {
    if (!d) return "—";
    try { return new Date(d).toLocaleDateString("en-GB", { day: "numeric", month: "long", year: "numeric" }); }
    catch { return d; }
  };

  if (loading) return (
    <div className="min-h-screen bg-background flex items-center justify-center">
      <Navbar /><Loader2 className="animate-spin text-primary" size={36} />
    </div>
  );

  return (
    <div className="min-h-screen flex flex-col bg-background">
      <Navbar />
      <main className="flex-1 container pt-24 pb-16 max-w-2xl mx-auto">

        {/* Avatar + name */}
        <div className="flex items-center gap-5 mb-8">
          <div className="w-16 h-16 rounded-full bg-primary/20 border-2 border-primary/40 flex items-center justify-center shrink-0">
            <span className="font-heading text-2xl text-primary">{initials}</span>
          </div>
          <div>
            <h1 className="font-heading text-2xl text-foreground">{fullName}</h1>
            <p className="text-sm text-muted-foreground">{profile?.phone_number}</p>
          </div>
          <button onClick={signOut}
            className="ml-auto flex items-center gap-1.5 text-sm text-muted-foreground hover:text-primary border border-border hover:border-primary/50 px-3 py-2 rounded-lg transition">
            <LogOut size={14} /> Sign Out
          </button>
        </div>

        {/* Profile card */}
        <div className="bg-card border border-border rounded-2xl overflow-hidden mb-6">
          <div className="flex items-center justify-between px-5 py-4 border-b border-border">
            <div className="flex items-center gap-2">
              <User size={15} className="text-primary" />
              <span className="font-heading text-sm tracking-wide">PERSONAL DETAILS</span>
            </div>
            {!editing ? (
              <button onClick={() => setEditing(true)}
                className="flex items-center gap-1.5 text-xs text-primary border border-primary/30 px-3 py-1.5 rounded-lg hover:bg-primary/10 transition">
                <Edit2 size={12} /> Edit
              </button>
            ) : (
              <div className="flex gap-2">
                <button onClick={() => { setEditing(false); setForm({ first_name: profile?.first_name || "", last_name: profile?.last_name || "", email: profile?.email || "" }); }}
                  className="flex items-center gap-1 text-xs text-muted-foreground border border-border px-3 py-1.5 rounded-lg hover:border-primary/50 transition">
                  <X size={12} /> Cancel
                </button>
                <button onClick={handleSave} disabled={saving}
                  className="flex items-center gap-1 text-xs bg-primary text-white px-3 py-1.5 rounded-lg hover:brightness-110 transition disabled:opacity-60">
                  {saving ? <Loader2 size={12} className="animate-spin" /> : <Check size={12} />} Save
                </button>
              </div>
            )}
          </div>

          <div className="divide-y divide-border">
            {/* Full Name */}
            <div className="flex items-center gap-3 px-5 py-4">
              <User size={15} className="text-muted-foreground shrink-0" />
              <div className="flex-1">
                <p className="text-xs text-muted-foreground mb-0.5">Full Name</p>
                {editing ? (
                  <div className="flex gap-2">
                    <input value={form.first_name} onChange={e => setForm(f => ({ ...f, first_name: e.target.value }))}
                      placeholder="First name"
                      className="flex-1 bg-secondary text-foreground border border-border rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-primary transition" />
                    <input value={form.last_name} onChange={e => setForm(f => ({ ...f, last_name: e.target.value }))}
                      placeholder="Last name"
                      className="flex-1 bg-secondary text-foreground border border-border rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-primary transition" />
                  </div>
                ) : (
                  <p className="text-sm font-medium text-foreground">{fullName}</p>
                )}
              </div>
            </div>

            {/* Email */}
            <div className="flex items-start gap-3 px-5 py-4">
              <Mail size={15} className="text-muted-foreground shrink-0 mt-0.5" />
              <div className="flex-1">
                <p className="text-xs text-muted-foreground mb-0.5">Email Address</p>
                {editing ? (
                  <input value={form.email} onChange={e => setForm(f => ({ ...f, email: e.target.value }))}
                    type="email" placeholder="your@email.com"
                    className="w-full bg-secondary text-foreground border border-border rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-primary transition" />
                ) : (
                  <p className="text-sm font-medium text-foreground">{profile?.email || <span className="text-muted-foreground italic">Not set</span>}</p>
                )}
                {!editing && !emailVerified && profile?.email && (
                  emailOtpSent ? (
                    <div className="mt-2 space-y-1.5">
                      <div className="flex gap-2">
                        <input value={emailOtp} onChange={e => setEmailOtp(e.target.value)} placeholder="Enter code"
                          maxLength={6}
                          className="w-28 bg-secondary text-foreground border border-border rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-primary transition" />
                        <button onClick={verifyEmailOtp} disabled={verifyingEmail}
                          className="flex items-center gap-1 text-xs bg-primary text-white px-3 py-1.5 rounded-lg hover:brightness-110 transition disabled:opacity-60">
                          {verifyingEmail ? <Loader2 size={11} className="animate-spin" /> : <Check size={11} />} Verify
                        </button>
                      </div>
                      <button onClick={sendEmailOtp} className="text-xs text-muted-foreground hover:text-primary transition">
                        Resend code
                      </button>
                    </div>
                  ) : (
                    <button onClick={sendEmailOtp}
                      className="mt-1.5 flex items-center gap-1 text-xs text-primary border border-primary/30 px-2.5 py-1 rounded-lg hover:bg-primary/10 transition">
                      <Send size={10} /> Send verification email
                    </button>
                  )
                )}
              </div>
              {emailVerified
                ? <span className="flex items-center gap-1 text-xs text-green-400 bg-green-400/10 border border-green-400/30 px-2 py-0.5 rounded-full shrink-0"><CheckCircle size={10} /> Verified</span>
                : <span className="flex items-center gap-1 text-xs text-yellow-400 bg-yellow-400/10 border border-yellow-400/30 px-2 py-0.5 rounded-full shrink-0"><AlertCircle size={10} /> Not Verified</span>
              }
            </div>

            {/* Phone */}
            <div className="flex items-start gap-3 px-5 py-4">
              <Phone size={15} className="text-muted-foreground shrink-0 mt-0.5" />
              <div className="flex-1">
                <p className="text-xs text-muted-foreground mb-0.5">Phone Number</p>
                <p className="text-sm font-medium text-foreground">{profile?.phone_number || "—"}</p>
                {!phoneVerified && profile?.phone_number && (
                  phoneOtpSent ? (
                    <div className="mt-2 space-y-1.5">
                      <div className="flex gap-2">
                        <input value={phoneOtp} onChange={e => setPhoneOtp(e.target.value)} placeholder="Enter code"
                          maxLength={6}
                          className="w-28 bg-secondary text-foreground border border-border rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-primary transition" />
                        <button onClick={verifyPhoneOtp} disabled={verifyingPhone}
                          className="flex items-center gap-1 text-xs bg-primary text-white px-3 py-1.5 rounded-lg hover:brightness-110 transition disabled:opacity-60">
                          {verifyingPhone ? <Loader2 size={11} className="animate-spin" /> : <Check size={11} />} Verify
                        </button>
                      </div>
                      <button onClick={sendPhoneOtp} className="text-xs text-muted-foreground hover:text-primary transition">
                        Resend code
                      </button>
                    </div>
                  ) : (
                    <button onClick={sendPhoneOtp}
                      className="mt-1.5 flex items-center gap-1 text-xs text-primary border border-primary/30 px-2.5 py-1 rounded-lg hover:bg-primary/10 transition">
                      <Send size={10} /> Send OTP
                    </button>
                  )
                )}
              </div>
              {phoneVerified
                ? <span className="flex items-center gap-1 text-xs text-green-400 bg-green-400/10 border border-green-400/30 px-2 py-0.5 rounded-full shrink-0"><CheckCircle size={10} /> Verified</span>
                : <span className="flex items-center gap-1 text-xs text-yellow-400 bg-yellow-400/10 border border-yellow-400/30 px-2 py-0.5 rounded-full shrink-0"><AlertCircle size={10} /> Not Verified</span>
              }
            </div>

            {/* Member since */}
            <div className="flex items-center gap-3 px-5 py-4">
              <Calendar size={15} className="text-muted-foreground shrink-0" />
              <div className="flex-1">
                <p className="text-xs text-muted-foreground mb-0.5">Member Since</p>
                <p className="text-sm font-medium text-foreground">{fmtDate(profile?.created_at)}</p>
              </div>
            </div>

            {/* KYC */}
            {profile?.kyc_status && (
              <div className="flex items-center gap-3 px-5 py-4">
                <Shield size={15} className="text-muted-foreground shrink-0" />
                <div className="flex-1">
                  <p className="text-xs text-muted-foreground mb-0.5">Verification Status</p>
                  <p className="text-sm font-medium text-foreground capitalize">{profile.kyc_status.toLowerCase()}</p>
                </div>
                <span className={`text-xs px-2 py-0.5 rounded-full border font-semibold ${
                  profile.kyc_status === "VERIFIED"
                    ? "text-green-400 bg-green-400/10 border-green-400/30"
                    : "text-yellow-400 bg-yellow-400/10 border-yellow-400/30"
                }`}>{profile.kyc_status === "VERIFIED" ? "✓ Verified" : "Pending"}</span>
              </div>
            )}
          </div>
        </div>

        {/* Quick links */}
        <div className="grid grid-cols-2 gap-3">
          <Link to="/my-tickets"
            className="bg-card border border-border rounded-xl p-4 flex items-center gap-3 hover:border-primary/50 transition group">
            <div className="w-9 h-9 rounded-lg bg-primary/10 flex items-center justify-center group-hover:bg-primary/20 transition">
              <Ticket size={16} className="text-primary" />
            </div>
            <div>
              <p className="text-sm font-semibold text-foreground">My Tickets</p>
              <p className="text-xs text-muted-foreground">View all entries</p>
            </div>
          </Link>
          <Link to="/competitions"
            className="bg-card border border-border rounded-xl p-4 flex items-center gap-3 hover:border-primary/50 transition group">
            <div className="w-9 h-9 rounded-lg bg-primary/10 flex items-center justify-center group-hover:bg-primary/20 transition">
              <Shield size={16} className="text-primary" />
            </div>
            <div>
              <p className="text-sm font-semibold text-foreground">Competitions</p>
              <p className="text-xs text-muted-foreground">Browse & enter</p>
            </div>
          </Link>
        </div>

      </main>
      <Footer />
    </div>
  );
};

export default ProfilePage;
