import { useState, useEffect, useMemo } from "react";
import { useParams, Link, useNavigate } from "react-router-dom";
import { motion } from "framer-motion";
import { ArrowLeft, Minus, Plus, Loader2, CheckCircle2, AlertCircle, Trophy, Smartphone, Copy, Check } from "lucide-react";
import Navbar from "@/components/Navbar";
import Footer from "@/components/Footer";
import { useCountdown } from "@/hooks/useCountdown";
import { fetchActiveGames, fetchGameSchedule, buyTicket, initiateDeposit, getToken, getPlayerId, type ApiGame, type ApiSchedule } from "@/lib/api";
import { toast } from "@/hooks/use-toast";

// ── MoMo config — update with real merchant number ────────────────────────────
const MOMO_NUMBER  = "0244000000";
const MOMO_NAME    = "WinBig Africa";
const MOMO_NETWORK = "MTN MoMo";

type PayState = "idle" | "awaiting_payment" | "processing" | "success" | "error";

const CompetitionDetail = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();

  const [game, setGame]         = useState<ApiGame | null>(null);
  const [schedule, setSchedule] = useState<ApiSchedule | null>(null);
  const [loading, setLoading]   = useState(true);
  const [qty, setQty]           = useState(1);
  const [payState, setPayState] = useState<PayState>("idle");
  const [tickets, setTickets]   = useState<string[]>([]);
  const [errMsg, setErrMsg]     = useState("");
  const [copied, setCopied]     = useState(false);
  const [momoPhone, setMomoPhone] = useState("");
  const [momoNetwork, setMomoNetwork] = useState("MTN");

  const isLoggedIn = !!getToken();
  const playerId   = getPlayerId();

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    fetchActiveGames()
      .then(games => {
        const g = games.find(g => g.id === id || g.code === id);
        if (!g) { setLoading(false); return; }
        setGame(g);
        return fetchGameSchedule(g.id);
      })
      .then(schedules => {
        if (!schedules) return;
        const s = (schedules as ApiSchedule[]).find(s => s.status === "SCHEDULED" || s.is_active)
          ?? (schedules as ApiSchedule[])[0]
          ?? null;
        setSchedule(s);
      })
      .catch(console.error)
      .finally(() => setLoading(false));
  }, [id]);

  const drawDate = useMemo(() => {
    if (game?.draw_date) {
      return new Date(game.draw_date + "T" + (game.draw_time || "20:00") + ":00Z");
    }
    // Daily/weekly — next draw at draw_time today or tomorrow
    const [h, m] = (game?.draw_time || "20:00").split(":").map(Number);
    const now = new Date();
    const next = new Date(now);
    next.setUTCHours(h, m, 0, 0);
    if (next <= now) next.setUTCDate(next.getUTCDate() + 1);
    return next;
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [game?.id, game?.draw_date, game?.draw_time]);
  const { days, hours, minutes, seconds } = useCountdown(drawDate);

  const copyMoMo = () => {
    navigator.clipboard.writeText(MOMO_NUMBER).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  };

  const handleConfirmPayment = async () => {
    if (!playerId || !game || !schedule) return;
    setPayState("processing");
    setErrMsg("");
    try {
      // Record the payment transaction first
      const phone = momoPhone || "+233256826832"; // fallback to registered phone
      await initiateDeposit(playerId, {
        amount: Math.round(game.base_price * qty * 100), // total in pesewas
        mobile_money_phone: phone,
        payment_method: momoNetwork,
        customer_name: "WinBig Player",
      }).catch(() => {}); // don't block ticket issue if deposit recording fails

      // Issue the tickets
      const results: string[] = [];
      for (let i = 0; i < qty; i++) {
        const r = await buyTicket(playerId, {
          game_code: game.code,
          game_schedule_id: schedule.id,
          draw_number: schedule.draw_number ?? 1,
          bet_lines: [{ line_number: 1, bet_type: "RAFFLE", total_amount: Math.round(game.base_price * 100) }],
        });
        results.push(r?.ticket?.serial_number || r?.serial_number || `TKT-${Date.now()}`);
      }
      setTickets(results);
      setPayState("success");
    } catch (err: unknown) {
      const msg = (err as Error).message || "Could not issue ticket";
      setErrMsg(msg);
      setPayState("error");
      toast({ title: "Ticket issue failed", description: msg, variant: "destructive" });
    }
  };

  if (loading) return (
    <div className="min-h-screen bg-background flex items-center justify-center">
      <Navbar /><Loader2 className="animate-spin text-primary" size={40} />
    </div>
  );

  if (!game) return (
    <div className="min-h-screen bg-background">
      <Navbar />
      <div className="container pt-24 text-center">
        <h1 className="font-heading text-3xl text-foreground mb-4">Competition Not Found</h1>
        <Link to="/competitions" className="text-primary hover:underline">← Back to Competitions</Link>
      </div>
    </div>
  );

  let prizeLabel = "";
  try { const p = JSON.parse(game.prize_details || "[]"); if (p[0]?.description) prizeLabel = p[0].description; } catch { /* */ }

  const price   = game.base_price;
  const total   = (qty * price).toFixed(2);
  const maxQty  = game.max_tickets_per_player || 10;
  const hasSchedule = !!schedule;
  const ref     = `${game.code}-${qty}TKT`;
  const timeLeft = days > 0
    ? `${days}d ${String(hours).padStart(2,"0")}h`
    : `${String(hours).padStart(2,"0")}:${String(minutes).padStart(2,"0")}:${String(seconds).padStart(2,"0")}`;

  return (
    <div className="min-h-screen bg-background">
      <Navbar />
      <div className="container pt-24 pb-16">
        <Link to="/competitions" className="inline-flex items-center gap-2 text-muted-foreground hover:text-primary mb-6 text-sm">
          <ArrowLeft size={16} /> Back to Competitions
        </Link>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
          {/* Image */}
          <motion.div initial={{ opacity:0, x:-20 }} animate={{ opacity:1, x:0 }}>
            <div className="rounded-xl overflow-hidden border border-border bg-card">
              {game.logo_url
                ? <img src={game.logo_url} alt={game.name} className="w-full aspect-[4/3] object-cover" />
                : <div className="w-full aspect-[4/3] flex items-center justify-center bg-secondary"><Trophy size={80} className="text-primary/30" /></div>
              }
            </div>
          </motion.div>

          {/* Right panel */}
          <motion.div initial={{ opacity:0, x:20 }} animate={{ opacity:1, x:0 }} className="flex flex-col">
            <span className="self-start px-3 py-1 rounded-full text-xs font-bold uppercase mb-4 bg-green-500/20 text-green-400">LIVE</span>
            <h1 className="font-heading text-3xl md:text-4xl text-foreground mb-2">{game.name}</h1>
            {prizeLabel && <p className="text-primary font-semibold mb-2">🏆 {prizeLabel}</p>}
            {game.description && <p className="text-muted-foreground mb-5 text-sm">{game.description}</p>}

            {/* Countdown */}
            <div className="flex gap-3 mb-4">
              {days > 0 && <div className="bg-card border border-border rounded-lg px-3 py-2 text-center"><div className="font-heading text-xl text-primary">{String(days).padStart(2,"0")}</div><div className="text-[10px] text-muted-foreground">DAYS</div></div>}
              {[{l:"HRS",v:hours},{l:"MIN",v:minutes},{l:"SEC",v:seconds}].map(t => (
                <div key={t.l} className="bg-card border border-border rounded-lg px-3 py-2 text-center">
                  <div className="font-heading text-xl text-primary">{String(t.v).padStart(2,"0")}</div>
                  <div className="text-[10px] text-muted-foreground">{t.l}</div>
                </div>
              ))}
            </div>
            {game.draw_date && <p className="text-xs text-muted-foreground mb-5">Draw: {new Date(game.draw_date).toLocaleDateString("en-GB",{day:"numeric",month:"long",year:"numeric"})} at {game.draw_time}</p>}

            {/* Purchase card */}
            <div className="bg-card border border-border rounded-xl p-6 mt-auto">

              {/* ── SUCCESS ── */}
              {payState === "success" && (
                <div className="text-center">
                  <CheckCircle2 className="text-green-400 mx-auto mb-3" size={48} />
                  <h3 className="font-heading text-xl text-foreground mb-2">You're in! Good Luck 🎉</h3>
                  <p className="text-muted-foreground text-sm mb-4">{tickets.length} ticket{tickets.length > 1 ? "s" : ""} confirmed.</p>
                  <div className="flex flex-wrap gap-2 justify-center mb-4">
                    {tickets.map(t => <span key={t} className="bg-primary/10 text-primary border border-primary/20 px-3 py-1 rounded-md text-sm font-mono">{t}</span>)}
                  </div>
                  <p className="text-xs text-muted-foreground mb-4">Draw in {timeLeft}</p>
                  <Link to="/my-tickets" className="text-primary text-sm hover:underline">View My Tickets →</Link>
                </div>
              )}

              {/* ── AWAITING MOMO PAYMENT ── */}
              {(payState === "awaiting_payment" || payState === "processing" || payState === "error") && (
                <div className="space-y-4">
                  <div className="flex items-center gap-2">
                    <Smartphone size={18} className="text-primary" />
                    <h3 className="font-heading text-base text-foreground">Pay via {MOMO_NETWORK}</h3>
                  </div>

                  <div className="bg-secondary rounded-xl p-4 space-y-3">
                    <div className="flex items-center justify-between">
                      <div>
                        <p className="text-xs text-muted-foreground">Send to</p>
                        <p className="font-mono text-xl font-bold text-foreground">{MOMO_NUMBER}</p>
                        <p className="text-xs text-muted-foreground">{MOMO_NAME}</p>
                      </div>
                      <button onClick={copyMoMo} className="flex items-center gap-1.5 text-xs text-primary border border-primary/30 px-3 py-1.5 rounded-lg hover:bg-primary/10 transition">
                        {copied ? <><Check size={12} /> Copied!</> : <><Copy size={12} /> Copy</>}
                      </button>
                    </div>
                    <div className="border-t border-border pt-3 flex items-center justify-between">
                      <p className="text-xs text-muted-foreground">Amount</p>
                      <p className="font-heading text-2xl text-primary">GHS {total}</p>
                    </div>
                    <p className="text-xs text-muted-foreground">Reference: <span className="font-mono text-foreground">{ref}</span></p>
                  </div>

                  <ol className="text-xs text-muted-foreground space-y-1 list-decimal list-inside">
                    <li>Open MoMo app or dial <strong className="text-foreground">*170#</strong></li>
                    <li>Send <strong className="text-foreground">GHS {total}</strong> to <strong className="text-foreground">{MOMO_NUMBER}</strong></li>
                    <li>Use reference: <span className="font-mono text-foreground">{ref}</span></li>
                    <li>Come back and tap <strong className="text-foreground">"I Have Paid"</strong></li>
                  </ol>

                  {/* Phone + network for transaction record */}
                  <div className="space-y-2">
                    <p className="text-xs text-muted-foreground font-medium">Your MoMo details (for receipt)</p>
                    <div className="flex gap-2">
                      <select value={momoNetwork} onChange={e => setMomoNetwork(e.target.value)}
                        className="bg-secondary text-foreground border border-border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary transition w-28">
                        <option value="MTN">MTN</option>
                        <option value="TELECEL">Telecel</option>
                        <option value="AIRTELTIGO">AirtelTigo</option>
                      </select>
                      <input type="tel" placeholder="Your MoMo number" value={momoPhone}
                        onChange={e => setMomoPhone(e.target.value)}
                        className="flex-1 bg-secondary text-foreground placeholder:text-muted-foreground border border-border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary transition" />
                    </div>
                  </div>

                  {payState === "error" && errMsg && (
                    <div className="flex items-center gap-2 text-red-400 text-xs bg-red-400/10 rounded-lg px-3 py-2">
                      <AlertCircle size={13} /> {errMsg}
                    </div>
                  )}

                  <div className="flex gap-3">
                    <button onClick={() => { setPayState("idle"); setErrMsg(""); }}
                      className="flex-1 border border-border text-muted-foreground py-3 rounded-lg text-sm hover:border-primary/50 hover:text-foreground transition">
                      Cancel
                    </button>
                    <button onClick={handleConfirmPayment} disabled={payState === "processing"}
                      className="flex-1 bg-green-500 text-white font-heading py-3 rounded-lg hover:brightness-110 transition disabled:opacity-60 flex items-center justify-center gap-2">
                      {payState === "processing"
                        ? <><Loader2 className="animate-spin" size={16} /> Confirming...</>
                        : <><CheckCircle2 size={16} /> I Have Paid</>}
                    </button>
                  </div>
                </div>
              )}

              {/* ── IDLE — quantity + buy button ── */}
              {payState === "idle" && (
                <>
                  <div className="flex items-center justify-between mb-4">
                    <span className="text-muted-foreground text-sm">Ticket Price</span>
                    <span className="text-primary font-heading text-2xl">GHS {price.toFixed(2)}</span>
                  </div>

                  <div className="flex items-center justify-center gap-4 mb-3">
                    <button onClick={() => setQty(Math.max(1, qty-1))} className="w-10 h-10 rounded-full bg-secondary flex items-center justify-center hover:bg-border transition">
                      <Minus size={18} className="text-foreground" />
                    </button>
                    <span className="font-heading text-3xl text-foreground w-16 text-center">{qty}</span>
                    <button onClick={() => setQty(Math.min(maxQty, qty+1))} className="w-10 h-10 rounded-full bg-secondary flex items-center justify-center hover:bg-border transition">
                      <Plus size={18} className="text-foreground" />
                    </button>
                  </div>
                  <p className="text-xs text-muted-foreground text-center mb-1">Max {maxQty} per player</p>

                  <div className="text-center text-muted-foreground text-sm mb-4">
                    Total: <span className="text-primary font-bold text-lg">GHS {total}</span>
                    <span className="flex items-center justify-center gap-1 text-xs mt-0.5"><Smartphone size={11} /> Pay via MoMo</span>
                  </div>

                  {!hasSchedule && (
                    <div className="text-xs text-yellow-400 bg-yellow-400/10 rounded-lg px-3 py-2 mb-3 text-center">
                      Ticket sales not yet open for this competition
                    </div>
                  )}

                  <button
                    onClick={() => { if (!isLoggedIn) { navigate("/sign-in"); return; } setPayState("awaiting_payment"); }}
                    disabled={!hasSchedule}
                    className="w-full bg-primary text-white font-heading text-lg py-4 rounded-lg btn-glow hover:brightness-110 transition disabled:opacity-60"
                  >
                    {!isLoggedIn ? "SIGN IN TO BUY" : "BUY TICKETS"}
                  </button>

                  {!isLoggedIn && (
                    <p className="text-center text-xs text-muted-foreground mt-2">
                      <Link to="/sign-in" className="text-primary hover:underline">Sign in</Link> or <Link to="/sign-up" className="text-primary hover:underline">create account</Link>
                    </p>
                  )}
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
