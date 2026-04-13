import { useState } from "react";
import { useParams, Link } from "react-router-dom";
import { motion } from "framer-motion";
import { ArrowLeft, Minus, Plus, Loader2, CheckCircle2 } from "lucide-react";
import Navbar from "@/components/Navbar";
import Footer from "@/components/Footer";
import { competitions } from "@/lib/competitions";
import { useCountdown } from "@/hooks/useCountdown";

type PaymentState = "idle" | "processing" | "waiting" | "success" | "error";

const CompetitionDetail = () => {
  const { id } = useParams();
  const comp = competitions.find((c) => c.id === id);
  const [qty, setQty] = useState(1);
  const [payState, setPayState] = useState<PaymentState>("idle");
  const [ticketNumbers, setTicketNumbers] = useState<string[]>([]);

  const fallbackDate = new Date(Date.now() + 3600000);
  const { hours, minutes, seconds } = useCountdown(comp?.endsAt ?? fallbackDate);

  if (!comp) {
    return (
      <div className="min-h-screen bg-background">
        <Navbar />
        <div className="container pt-24 text-center">
          <h1 className="font-heading text-3xl text-foreground mb-4">Competition Not Found</h1>
          <Link to="/competitions" className="text-primary hover:underline">← Back to Competitions</Link>
        </div>
      </div>
    );
  }

  const pct = Math.round((comp.soldTickets / comp.totalTickets) * 100);
  const total = (qty * comp.ticketPrice).toFixed(2);

  const handleBuy = () => {
    setPayState("processing");
    setTimeout(() => setPayState("waiting"), 1500);
    setTimeout(() => {
      const tickets = Array.from({ length: qty }, () => `WBX-${Math.floor(10000 + Math.random() * 89999)}`);
      setTicketNumbers(tickets);
      setPayState("success");
    }, 4000);
  };

  return (
    <div className="min-h-screen bg-background">
      <Navbar />
      <div className="container pt-24 pb-16">
        <Link to="/competitions" className="inline-flex items-center gap-2 text-muted-foreground hover:text-primary mb-6 text-sm">
          <ArrowLeft size={16} /> Back to Competitions
        </Link>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
          <motion.div initial={{ opacity: 0, x: -20 }} animate={{ opacity: 1, x: 0 }}>
            <div className="rounded-xl overflow-hidden border border-border">
              <img src={comp.image} alt={comp.title} className="w-full aspect-[4/3] object-cover" width={800} height={600} />
            </div>
          </motion.div>

          <motion.div initial={{ opacity: 0, x: 20 }} animate={{ opacity: 1, x: 0 }} className="flex flex-col">
            <span className={`self-start px-3 py-1 rounded-full text-xs font-bold uppercase mb-4 ${
              comp.tag === "Ending Soon" ? "bg-accent/20 text-accent" : "bg-green-500/20 text-green-400"
            }`}>
              {comp.tag}
            </span>
            <h1 className="font-heading text-3xl md:text-4xl text-foreground mb-3">{comp.title}</h1>
            <p className="text-muted-foreground mb-6">{comp.description}</p>

            <div className="flex gap-3 mb-6">
              {[{ l: "HRS", v: hours }, { l: "MIN", v: minutes }, { l: "SEC", v: seconds }].map((t) => (
                <div key={t.l} className="bg-card border border-border rounded-lg px-3 py-2 text-center">
                  <div className="font-heading text-xl text-primary">{String(t.v).padStart(2, "0")}</div>
                  <div className="text-[10px] text-muted-foreground">{t.l}</div>
                </div>
              ))}
            </div>

            <div className="mb-4">
              <div className="flex justify-between text-sm mb-1">
                <span className="text-muted-foreground">{pct}% sold</span>
                <span className="text-muted-foreground">{comp.soldTickets}/{comp.totalTickets} tickets</span>
              </div>
              <div className="h-3 bg-secondary rounded-full overflow-hidden">
                <div className="h-full rounded-full bg-gradient-to-r from-primary to-accent" style={{ width: `${pct}%` }} />
              </div>
            </div>

            <div className="bg-card border border-border rounded-xl p-6 mt-auto">
              {payState === "success" ? (
                <div className="text-center">
                  <CheckCircle2 className="text-green-400 mx-auto mb-3" size={48} />
                  <h3 className="font-heading text-xl text-foreground mb-2">Good Luck! 🎉</h3>
                  <p className="text-muted-foreground text-sm mb-4">Your tickets are confirmed.</p>
                  <div className="flex flex-wrap gap-2 justify-center mb-3">
                    {ticketNumbers.map((t) => (
                      <span key={t} className="bg-primary/10 text-primary border border-primary/20 px-3 py-1 rounded-md text-sm font-mono">{t}</span>
                    ))}
                  </div>
                  <p className="text-muted-foreground text-xs">Draw date: {comp.endsAt.toLocaleDateString()}</p>
                </div>
              ) : (
                <>
                  <div className="flex items-center justify-between mb-4">
                    <span className="text-muted-foreground text-sm">Ticket Price</span>
                    <span className="text-primary font-heading text-2xl">{comp.currency}{comp.ticketPrice.toFixed(2)}</span>
                  </div>
                  <div className="flex items-center justify-center gap-4 mb-4">
                    <button onClick={() => setQty(Math.max(1, qty - 1))} className="w-10 h-10 rounded-full bg-secondary flex items-center justify-center hover:bg-border transition" disabled={payState !== "idle"}>
                      <Minus size={18} className="text-foreground" />
                    </button>
                    <span className="font-heading text-3xl text-foreground w-16 text-center">{qty}</span>
                    <button onClick={() => setQty(qty + 1)} className="w-10 h-10 rounded-full bg-secondary flex items-center justify-center hover:bg-border transition" disabled={payState !== "idle"}>
                      <Plus size={18} className="text-foreground" />
                    </button>
                  </div>
                  <div className="text-center text-muted-foreground text-sm mb-4">
                    Total: <span className="text-primary font-bold text-lg">{comp.currency}{total}</span>
                  </div>
                  <button
                    onClick={handleBuy}
                    disabled={payState !== "idle"}
                    className="w-full bg-primary text-primary-foreground font-heading text-lg py-4 rounded-lg btn-glow hover:brightness-110 transition disabled:opacity-60 flex items-center justify-center gap-2"
                  >
                    {payState === "idle" && "BUY TICKETS"}
                    {payState === "processing" && <><Loader2 className="animate-spin" size={20} /> Payment Request Sent...</>}
                    {payState === "waiting" && <><Loader2 className="animate-spin" size={20} /> Waiting for confirmation...</>}
                    {payState === "error" && "Try Again"}
                  </button>
                </>
              )}
            </div>
          </motion.div>
        </div>
      </div>
      <Footer />
    </div>
  );
};

export default CompetitionDetail;
