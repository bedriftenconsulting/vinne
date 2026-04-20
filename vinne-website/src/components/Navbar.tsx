import { useState, useRef, useEffect } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Menu, X, Ticket, User, LogOut } from "lucide-react";
import logo from "@/assets/logo.png";

const useAuth = () => {
  const [token, setToken] = useState(() => localStorage.getItem("player_token"));
  useEffect(() => {
    const sync = () => setToken(localStorage.getItem("player_token"));
    window.addEventListener("storage", sync);
    return () => window.removeEventListener("storage", sync);
  }, []);
  return token;
};

// Decode player ID from JWT without a library
const getPlayerIdFromToken = (token: string | null): string | null => {
  if (!token) return null;
  try {
    const payload = JSON.parse(atob(token.split(".")[1]));
    return payload.user_id || payload.sub || null;
  } catch {
    return null;
  }
};

const Navbar = () => {
  const [open, setOpen] = useState(false);
  const [dropOpen, setDropOpen] = useState(false);
  const dropRef = useRef<HTMLDivElement>(null);
  const navigate = useNavigate();
  const token = useAuth();

  // Close dropdown on outside click
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (dropRef.current && !dropRef.current.contains(e.target as Node)) {
        setDropOpen(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  const signOut = () => {
    localStorage.removeItem("player_token");
    setDropOpen(false);
    navigate("/");
    window.dispatchEvent(new Event("storage"));
  };

  return (
    <nav className="fixed top-0 left-0 right-0 z-50 bg-background/90 backdrop-blur-md border-b border-border">
      <div className="container flex items-center justify-between h-16">
        {/* Logo */}
        <Link to="/" className="flex items-center gap-2 shrink-0">
          <img src={logo} alt="WinBig Africa" className="h-10 w-10" />
          <span className="font-heading text-xl text-primary">WINBIG AFRICA</span>
        </Link>

        {/* Desktop nav */}
        <div className="hidden md:flex items-center gap-6">
          <Link to="/competitions" className="text-foreground/80 hover:text-primary transition-colors text-sm font-medium">Competitions</Link>
          <Link to="/results" className="text-foreground/80 hover:text-primary transition-colors text-sm font-medium">Results</Link>
          <Link to="/faq" className="text-foreground/80 hover:text-primary transition-colors text-sm font-medium">FAQ</Link>

          {token ? (
            <div className="flex items-center gap-2 ml-2">
              {/* My Tickets */}
              <Link
                to="/my-tickets"
                className="flex items-center gap-1.5 border border-border text-foreground px-4 py-2 rounded-lg text-sm font-semibold hover:border-primary hover:text-primary transition"
              >
                <Ticket size={15} />
                My Tickets
              </Link>

              {/* Account dropdown */}
              <div className="relative" ref={dropRef}>
                <button
                  onClick={() => setDropOpen(v => !v)}
                  className="flex items-center gap-1.5 border border-border text-foreground px-4 py-2 rounded-lg text-sm font-semibold hover:border-primary hover:text-primary transition"
                >
                  <User size={15} />
                  Account
                </button>

                {dropOpen && (
                  <div className="absolute right-0 mt-2 w-48 bg-card border border-border rounded-xl shadow-xl overflow-hidden z-50">
                    <Link
                      to="/profile"
                      onClick={() => setDropOpen(false)}
                      className="flex items-center gap-2 px-4 py-3 text-sm font-semibold bg-primary text-white hover:brightness-110 transition"
                    >
                      <User size={15} />
                      Profile
                    </Link>
                    <Link
                      to="/my-tickets"
                      onClick={() => setDropOpen(false)}
                      className="flex items-center gap-2 px-4 py-3 text-sm text-foreground/70 hover:text-foreground hover:bg-secondary transition"
                    >
                      <Ticket size={15} />
                      My Tickets
                    </Link>
                    <div className="border-t border-border" />
                    <button
                      onClick={signOut}
                      className="w-full flex items-center gap-2 px-4 py-3 text-sm text-primary hover:bg-secondary transition"
                    >
                      <LogOut size={15} />
                      Sign Out
                    </button>
                  </div>
                )}
              </div>
            </div>
          ) : (
            <div className="flex items-center gap-2 ml-2">
              <Link to="/sign-in" className="border border-primary text-primary px-5 py-2 rounded-lg font-semibold text-sm hover:bg-primary/10 transition">
                Sign In
              </Link>
              <Link to="/sign-up" className="bg-primary text-white px-5 py-2 rounded-lg font-semibold text-sm btn-glow hover:brightness-110 transition">
                Sign Up
              </Link>
            </div>
          )}
        </div>

        <button onClick={() => setOpen(!open)} className="md:hidden text-foreground">
          {open ? <X size={24} /> : <Menu size={24} />}
        </button>
      </div>

      {/* Mobile menu */}
      {open && (
        <div className="md:hidden bg-card border-b border-border px-4 pb-4 flex flex-col gap-3">
          <Link to="/competitions" onClick={() => setOpen(false)} className="py-2 text-foreground/80 hover:text-primary">Competitions</Link>
          <Link to="/results" onClick={() => setOpen(false)} className="py-2 text-foreground/80 hover:text-primary">Results</Link>
          <Link to="/faq" onClick={() => setOpen(false)} className="py-2 text-foreground/80 hover:text-primary">FAQ</Link>

          {token ? (
            <div className="flex flex-col gap-2 pt-1 border-t border-border">
              <Link to="/profile" onClick={() => setOpen(false)} className="flex items-center gap-2 py-2 text-foreground/80 hover:text-primary">
                <User size={15} /> Profile
              </Link>
              <Link to="/my-tickets" onClick={() => setOpen(false)} className="flex items-center gap-2 py-2 text-foreground/80 hover:text-primary">
                <Ticket size={15} /> My Tickets
              </Link>
              <button onClick={() => { signOut(); setOpen(false); }} className="flex items-center gap-2 py-2 text-primary text-left">
                <LogOut size={15} /> Sign Out
              </button>
            </div>
          ) : (
            <div className="flex flex-col gap-2 pt-1">
              <Link to="/sign-in" onClick={() => setOpen(false)} className="border border-primary text-primary px-5 py-2 rounded-lg font-semibold text-sm text-center hover:bg-primary/10 transition">
                Sign In
              </Link>
              <Link to="/sign-up" onClick={() => setOpen(false)} className="bg-primary text-white px-5 py-2 rounded-lg font-semibold text-sm text-center btn-glow">
                Sign Up
              </Link>
            </div>
          )}
        </div>
      )}
    </nav>
  );
};

export default Navbar;
