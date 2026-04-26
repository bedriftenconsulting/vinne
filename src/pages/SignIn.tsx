import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { motion } from "framer-motion";
import { Eye, EyeOff, Loader2, Phone, Lock } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import Navbar from "@/components/Navbar";
import Footer from "@/components/Footer";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { toast } from "@/hooks/use-toast";

// Map backend errors to friendly messages
const friendlyError = (msg: string): string => {
  const m = msg.toLowerCase();
  if (m.includes('invalid credentials') || m.includes('invalid phone') || m.includes('unauthorized'))
    return 'Incorrect phone number or password. Please try again.';
  if (m.includes('suspended') || m.includes('banned'))
    return 'Your account has been suspended. Contact support for help.';
  if (m.includes('not found') || m.includes('no player'))
    return 'No account found with that phone number. Did you mean to sign up?';
  if (m.includes('network') || m.includes('fetch'))
    return 'Cannot reach the server. Check your connection and try again.';
  return msg || 'Sign in failed. Please try again.';
};

const SignIn = () => {
  const navigate = useNavigate();
  const { login } = useAuth();
  const [showPw, setShowPw] = useState(false);
  const [loading, setLoading] = useState(false);
  const [phone, setPhone]     = useState('');
  const [password, setPassword] = useState('');
  const [touched, setTouched] = useState({ phone: false, password: false });

  const handlePhoneChange = (value: string) => {
    const cleaned = value.replace(/[^\d+]/g, '');
    const normalized = cleaned.startsWith('0') && cleaned.length > 1
      ? '+233' + cleaned.substring(1)
      : cleaned;
    setPhone(normalized);
  };

  const phoneOk    = phone.replace(/\D/g, '').length === 10;
  const passwordOk = password.length >= 1;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setTouched({ phone: true, password: true });

    if (!phone) {
      toast({ title: 'Phone number required', description: 'Enter your registered phone number.', variant: 'destructive' });
      return;
    }
    if (!phoneOk) {
      toast({ title: 'Invalid phone number', description: 'Use your Ghana number in +233XXXXXXXXX format.', variant: 'destructive' });
      return;
    }
    if (!password) {
      toast({ title: 'Password required', description: 'Enter your password to sign in.', variant: 'destructive' });
      return;
    }

    setLoading(true);
    try {
      await login(phone, password);
      toast({ title: 'Welcome back!', description: 'You\'re now signed in.' });
      navigate('/');
    } catch (err: any) {
      console.error('[SignIn] login error:', err);
      toast({ title: 'Sign in failed', description: friendlyError(err.message), variant: 'destructive' });
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
              <CardTitle className="text-2xl font-heading">Welcome Back</CardTitle>
              <CardDescription>Sign in to your WinBig Africa account</CardDescription>
            </CardHeader>
            <CardContent>
              <form onSubmit={handleSubmit} className="space-y-4" noValidate>

                {/* Phone */}
                <div>
                  <Label htmlFor="phone">Phone Number</Label>
                  <div className="relative">
                    <Phone className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
                    <Input id="phone" type="tel" placeholder="+233201234567" value={phone}
                      onChange={e => handlePhoneChange(e.target.value)}
                      onBlur={() => setTouched(t => ({ ...t, phone: true }))}
                      className={`pl-10 ${touched.phone && !phoneOk ? 'border-red-500' : ''}`} />
                  </div>
                  {touched.phone && !phoneOk && (
                    <p className="text-xs text-red-500 mt-1">
                      Must be +233 followed by 9 digits (e.g. +233201234567)
                    </p>
                  )}
                </div>

                {/* Password */}
                <div>
                  <div className="flex items-center justify-between mb-1">
                    <Label htmlFor="password">Password</Label>
                    <Link to="/forgot-password" className="text-xs text-primary hover:underline">
                      Forgot password?
                    </Link>
                  </div>
                  <div className="relative">
                    <Lock className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
                    <Input id="password" type={showPw ? 'text' : 'password'} placeholder="Enter your password"
                      value={password} onChange={e => setPassword(e.target.value)}
                      onBlur={() => setTouched(t => ({ ...t, password: true }))}
                      className={`pl-10 pr-10 ${touched.password && !passwordOk ? 'border-red-500' : ''}`} />
                    <button type="button" onClick={() => setShowPw(p => !p)}
                      className="absolute right-3 top-2.5 text-muted-foreground hover:text-foreground transition-colors z-10">
                      {showPw ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                    </button>
                  </div>
                  {touched.password && !passwordOk && (
                    <p className="text-xs text-red-500 mt-1">Password is required</p>
                  )}
                </div>

                <Button type="submit" className="w-full h-11" disabled={loading}>
                  {loading ? <><Loader2 className="h-4 w-4 animate-spin mr-2" />Signing In...</> : 'Sign In'}
                </Button>

              </form>

              <div className="mt-6 text-center">
                <p className="text-sm text-muted-foreground">
                  Don't have an account?{' '}
                  <Link to="/signup" className="text-primary hover:underline">Create one here</Link>
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

export default SignIn;
