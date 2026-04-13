import { useState } from "react";
import { Link } from "react-router-dom";
import { Menu, X } from "lucide-react";
import logo from "@/assets/logo.png";

const Navbar = () => {
  const [open, setOpen] = useState(false);

  return (
    <nav className="fixed top-0 left-0 right-0 z-50 bg-background/90 backdrop-blur-md border-b border-border">
      <div className="container flex items-center justify-between h-16">
        {/* Logo → home */}
        <Link to="/" className="flex items-center gap-2 shrink-0">
          <img src={logo} alt="WinBig Africa" className="h-10 w-10" />
          <span className="font-heading text-xl text-primary">WINBIG AFRICA</span>
        </Link>

        {/* Desktop nav */}
        <div className="hidden md:flex items-center gap-6">
          <Link to="/competitions" className="text-foreground/80 hover:text-primary transition-colors text-sm font-medium">Competitions</Link>
          <Link to="/results" className="text-foreground/80 hover:text-primary transition-colors text-sm font-medium">Results</Link>
          <Link to="/faq" className="text-foreground/80 hover:text-primary transition-colors text-sm font-medium">FAQ</Link>
          <div className="flex items-center gap-2 ml-2">
            <Link
              to="/sign-in"
              className="border border-primary text-primary px-5 py-2 rounded-lg font-semibold text-sm hover:bg-primary/10 transition"
            >
              Sign In
            </Link>
            <Link
              to="/sign-up"
              className="bg-primary text-white px-5 py-2 rounded-lg font-semibold text-sm btn-glow hover:brightness-110 transition"
            >
              Sign Up
            </Link>
          </div>
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
          <div className="flex flex-col gap-2 pt-1">
            <Link
              to="/sign-in"
              onClick={() => setOpen(false)}
              className="border border-primary text-primary px-5 py-2 rounded-lg font-semibold text-sm text-center hover:bg-primary/10 transition"
            >
              Sign In
            </Link>
            <Link
              to="/sign-up"
              onClick={() => setOpen(false)}
              className="bg-primary text-white px-5 py-2 rounded-lg font-semibold text-sm text-center btn-glow"
            >
              Sign Up
            </Link>
          </div>
        </div>
      )}
    </nav>
  );
};

export default Navbar;
