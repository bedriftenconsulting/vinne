import { Link } from "react-router-dom";
import logo from "@/assets/logo.png";

const Footer = () => (
  <footer className="border-t border-border py-12 bg-card/30">
    <div className="container">
      <div className="grid grid-cols-1 md:grid-cols-4 gap-8 mb-8">
        <div>
          <div className="flex items-center gap-2 mb-3">
            <img src={logo} alt="WinBig Africa" className="h-8 w-8" />
            <span className="font-heading text-primary">WINBIG AFRICA</span>
          </div>
          <p className="text-muted-foreground text-sm">Africa's #1 Competition Platform. Win luxury prizes for as little as $0.39.</p>
        </div>
        <div>
          <h4 className="font-heading text-foreground mb-3">Quick Links</h4>
          <div className="flex flex-col gap-2 text-sm text-muted-foreground">
            <Link to="/competitions" className="hover:text-primary transition-colors">Competitions</Link>
            <Link to="/results" className="hover:text-primary transition-colors">Results</Link>
            <Link to="/faq" className="hover:text-primary transition-colors">FAQ</Link>
          </div>
        </div>
        <div>
          <h4 className="font-heading text-foreground mb-3">Support</h4>
          <div className="flex flex-col gap-2 text-sm text-muted-foreground">
            <span>📧 support@winbigafrica.com</span>
            <span>📞 +233 20 000 0000</span>
          </div>
        </div>
        <div>
          <h4 className="font-heading text-foreground mb-3">Legal</h4>
          <div className="flex flex-col gap-2 text-sm text-muted-foreground">
            <Link to="#" className="hover:text-primary transition-colors">Terms & Conditions</Link>
            <Link to="#" className="hover:text-primary transition-colors">Privacy Policy</Link>
            <Link to="#" className="hover:text-primary transition-colors">Responsible Play</Link>
          </div>
        </div>
      </div>
      <div className="border-t border-border pt-6 text-center text-muted-foreground text-xs">
        © 2026 WinBig Africa. All rights reserved. Play responsibly. 18+
      </div>
    </div>
  </footer>
);

export default Footer;
