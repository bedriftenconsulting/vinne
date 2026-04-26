import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { motion } from "framer-motion";
import { Eye, EyeOff, Loader2, User, Mail, Phone, Lock, CheckCircle2, XCircle } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import Navbar from "@/components/Navbar";
import Footer from "@/components/Footer";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { toast } from "@/hooks/use-toast";

// ── Validation helpers ────────────────────────────────────────────────────────

const isValidPhone = (p: string) => {
  // Remove all non-digits and check if it's 10 digits (Ghana format)
  const digits = p.replace(/\D/g, '');
  return digits.length === 10;
};
const isValidEmail = (e: string) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(e);
const isValidName  = (n: string) => n.trim().length >= 2;

// Map backend error messages to friendly text
const friendlyError = (msg: string): string => {
  const m = msg.toLowerCase();
  if (m.includes('phone number already registered') || m.includes('already registered'))
    return 'This phone number is already registered. Try signing in instead.';
  if (m.includes('phone') && m.includes('invalid'))
    return 'Invalid phone number format. Use +233XXXXXXXXX.';
  if (m.includes('value too long'))
    return 'Phone number is too long. Make sure it\'s in +233XXXXXXXXX format.';
  if (m.includes('password') && m.includes('weak'))
    return 'Password is too weak. Use at least 8 characters with letters and numbers.';
  if (m.includes('network') || m.includes('fetch'))
    return 'Cannot reach the server. Check your connection and try again.';
  return msg || 'Something went wrong. Please try again.';
};

// ── Field hint component ──────────────────────────────────────────────────────

const FieldHint = ({ show, ok, text }: { show: boolean; ok: boolean; text: string }) => {
  if (!show) return null;
  return (
    <p className={`flex items-center gap-1 text-xs mt-1 ${ok ? 'text-green-500' : 'text-red-500'}`}>
      {ok ? <CheckCircle2 className="h-3 w-3" /> : <XCircle className="h-3 w-3" />}
      {text}
    </p>
  );
};

// ── Component ─────────────────────────────────────────────────────────────────

interface Form { name: string; email: string; phone: string; password: string; confirm: string }
type Touched = Partial<Record<keyof Form, boolean>>;

const SignUp = () => {
  const navigate = useNavigate();
  const { register } = useAuth();
  const [showPw, setShowPw]   = useState(false);
  const [showCpw, setShowCpw] = useState(false);
  const [loading, setLoading] = useState(false);
  const [form, setForm]       = useState<Form>({ name: '', email: '', phone: '', password: '', confirm: '' });
  const [touched, setTouched] = useState<Touched>({});

  const touch = (field: keyof Form) => setTouched(t => ({ ...t, [field]: true }));

  const set = (field: keyof Form, value: string) => {
    if (field === 'phone') {
      const cleaned = value.replace(/[^\d+]/g, '');
      const normalized = cleaned.startsWith('0') && cleaned.length > 1
        ? '+233' + cleaned.substring(1)
        : cleaned;
      setForm(f => ({ ...f, phone: normalized }));
    } else {
      setForm(f => ({ ...f, [field]: value }));
    }
  };

  // Per-field validity
  const v = {
    name:    isValidName(form.name),
    email:   isValidEmail(form.email),
    phone:   isValidPhone(form.phone),
    password: form.password.length >= 8,
    confirm: form.confirm === form.password && form.confirm.length > 0,
  };

  const allValid = Object.values(v).every(Boolean);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    // Touch all fields to show errors
    setTouched({ name: true, email: true, phone: true, password: true, confirm: true });

    if (!v.name) {
      toast({ title: 'Full name required', description: 'Enter your first and last name.', variant: 'destructive' });
      return;
    }
    if (!v.email) {
      toast({ title: 'Invalid email', description: 'Enter a valid email address like you@example.com.', variant: 'destructive' });
      return;
    }
    if (!v.phone) {
      toast({ title: 'Invalid phone number', description: 'Use your Ghana number in +233XXXXXXXXX format (9 digits after +233).', variant: 'destructive' });
      return;
    }
    if (!v.password) {
      toast({ title: 'Password too short', description: 'Password must be at least 8 characters.', variant: 'destructive' });
      return;
    }
    if (!v.confirm) {
      toast({ title: 'Passwords don\'t match', description: 'Make sure both password fields are identical.', variant: 'destructive' });
      return;
    }

    setLoading(true);
    try {
      const res = await register({ name: form.name, email: form.email, phone: form.phone, password: form.password });
      toast({
        title: '🎉 Account created!',
        description: res.requires_otp ? 'Check your phone for a verification code.' : 'You can now sign in.',
      });
      navigate('/signin');
    } catch (err: any) {
      toast({ title: 'Registration failed', description: friendlyError(err.message), variant: 'destructive' });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-background">
      <Navbar />
      <div className="container pt-24 pb-16">
        <motion.div initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} className="max-w-md mx-auto">
          <Card>
            <CardHeader className="text-center">
              <CardTitle className="text-2xl font-heading">Create Account</CardTitle>
              <CardDescription>Join WinBig Africa and start winning amazing prizes!</CardDescription>
            </CardHeader>
            <CardContent>
              <form onSubmit={handleSubmit} className="space-y-4" noValidate>

                {/* Full Name */}
                <div>
                  <Label htmlFor="name">Full Name</Label>
                  <div className="relative">
                    <User className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
                    <Input id="name" placeholder="John Doe" value={form.name}
                      onChange={e => set('name', e.target.value)} onBlur={() => touch('name')}
                      className={`pl-10 ${touched.name && !v.name ? 'border-red-500' : ''}`} />
                  </div>
                  <FieldHint show={!!touched.name} ok={v.name} text={v.name ? 'Looks good!' : 'Enter your first and last name'} />
                </div>

                {/* Email */}
                <div>
                  <Label htmlFor="email">Email Address</Label>
                  <div className="relative">
                    <Mail className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
                    <Input id="email" type="email" placeholder="you@example.com" value={form.email}
                      onChange={e => set('email', e.target.value)} onBlur={() => touch('email')}
                      className={`pl-10 ${touched.email && !v.email ? 'border-red-500' : ''}`} />
                  </div>
                  <FieldHint show={!!touched.email} ok={v.email} text={v.email ? 'Valid email' : 'Enter a valid email like you@example.com'} />
                </div>

                {/* Phone */}
                <div>
                  <Label htmlFor="phone">Phone Number</Label>
                  <div className="relative">
                    <Phone className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
                    <Input id="phone" type="tel" placeholder="+233201234567" value={form.phone}
                      onChange={e => set('phone', e.target.value)} onBlur={() => touch('phone')}
                      className={`pl-10 ${touched.phone && !v.phone ? 'border-red-500' : ''}`} />
                  </div>
                  <FieldHint show={!!touched.phone} ok={v.phone}
                    text={v.phone ? 'Valid Ghana number' : 'Must be +233 followed by 9 digits (e.g. +233201234567)'} />
                </div>

                {/* Password */}
                <div>
                  <Label htmlFor="password">Password</Label>
                  <div className="relative">
                    <Lock className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
                    <Input id="password" type={showPw ? 'text' : 'password'} placeholder="Min. 8 characters"
                      value={form.password} onChange={e => set('password', e.target.value)} onBlur={() => touch('password')}
                      className={`pl-10 pr-10 ${touched.password && !v.password ? 'border-red-500' : ''}`} />
                    <button type="button" onClick={() => setShowPw(p => !p)}
                      className="absolute right-3 top-2.5 text-muted-foreground hover:text-foreground transition-colors">
                      {showPw ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                    </button>
                  </div>
                  <FieldHint show={!!touched.password} ok={v.password} text={v.password ? 'Strong enough' : 'At least 8 characters required'} />
                </div>

                {/* Confirm Password */}
                <div>
                  <Label htmlFor="confirm">Confirm Password</Label>
                  <div className="relative">
                    <Lock className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
                    <Input id="confirm" type={showCpw ? 'text' : 'password'} placeholder="Repeat your password"
                      value={form.confirm} onChange={e => set('confirm', e.target.value)} onBlur={() => touch('confirm')}
                      className={`pl-10 pr-10 ${touched.confirm && !v.confirm ? 'border-red-500' : ''}`} />
                    <button type="button" onClick={() => setShowCpw(p => !p)}
                      className="absolute right-3 top-2.5 text-muted-foreground hover:text-foreground transition-colors">
                      {showCpw ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                    </button>
                  </div>
                  <FieldHint show={!!touched.confirm} ok={v.confirm} text={v.confirm ? 'Passwords match' : 'Passwords do not match'} />
                </div>

                <Button type="submit" className="w-full h-11" disabled={loading}>
                  {loading ? <><Loader2 className="h-4 w-4 animate-spin mr-2" />Creating Account...</> : 'Create Account'}
                </Button>

              </form>

              <div className="mt-6 text-center">
                <p className="text-sm text-muted-foreground">
                  Already have an account?{' '}
                  <Link to="/signin" className="text-primary hover:underline">Sign in here</Link>
                </p>
              </div>
            </CardContent>
          </Card>
        </motion.div>
      </div>
      <Footer />
    </div>
  );
};

export default SignUp;
