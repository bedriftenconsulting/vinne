import { useState, useEffect, useMemo } from "react";
import { useParams, Link, useNavigate } from "react-router-dom";
import { motion } from "framer-motion";
import { ArrowLeft, Minus, Plus, Loader2, CheckCircle2, AlertCircle, Trophy, Smartphone, Phone } from "lucide-react";
import Navbar from "@/components/Navbar";
import Footer from "@/components/Footer";
import { useCountdown } from "@/hooks/useCountdown";
import { fetchActiveGames, fetchGameSchedule, getToken, getPlayerId, type ApiGame, type ApiSchedule } from "@/lib/api";
import { toast } from "@/hooks/use-toast";

type PayState = "idle" | "momo_input" | "waiting" | "success" | "error";

const NETWORKS = [
  { id: "MTN",        label: "MTN MoMo" },
  { id: "TELECEL",    label: "Telecel Cash" },
  { id: "AIRTELTIGO", label: "AirtelTigo" },
];

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
  const [momoPhone, setMomoPhone]   = useState("");
  const [momoNetwork, setMomoNetwork] = useState("MTN");
  const [txRef, setTxRef]       = useState("");

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
    const [h, m] = (game?.draw_time || "20:00").split(":").map(Number);
    const now = new Date();
    const next = new Date(now);
    next.setUTCHours(h, m, 0, 0);
    if (next <= now) next.setUTCDate(next.getUTCDate() + 1);
    return next;
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [game?.id, game?.draw_date, game?.draw_time]);
  const { days, hours, minutes, seconds } = useCountdown(drawDate);

  // Create tickets + fire Hubtel STK push
  const handleInitiatePayment = async () => {
    if (!playerId) {
      setErrMsg("Session expired — please sign out and sign in again.");
      return;
    }
    if (!game || !schedule) return;

    const phone = momoPhone.replace(/\s+/g, "");
    if (phone.replace(/\D/g, '').length !== 10) {
      setErrMsg("Enter a valid 10-digit Ghana phone number");
      return;
    }

    setPayState("waiting");
    setErrMsg("");

    try {
      const res = await fetch("/hubtel/web/payment/initiate", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          msisdn:           phone,
          qty,
          network:          momoNetwork,
          player_id:        playerId,
          game_code:        game.code,
          game_schedule_id: schedule.id,
          draw_number:      schedule.draw_number ?? 1,
          game_name:        game.name,
          unit_price:       Math.round(game.base_price * 100),
        }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || "Payment initiation failed");

      setTxRef(data.reference || "");
      pollHubtelStatus(data.reference, data.serials || []);
    } catch (err: unknown) {
      const msg = (err as Error).message || "Payment failed. Please try again.";
      setErrMsg(msg);
      setPayState("momo_input");
      toast({ title: "Payment failed", description: msg, variant: "destructive" });
    }
  };

  // Poll every 5s — Hubtel webhook updates DB when payment confirmed
  const pollHubtelStatus = (reference: string, pendingSerials: string[]) => {
    const maxAttempts = 24; // 2 minutes
    let attempts = 0;

    const poll = async () => {
      attempts++;
      try {
        const res = await fetch(`/hubtel/web/payment/status/${reference}`);
        const data = await res.json();
        const status = data?.status || "";

        if (status === "completed") {
          setTickets(data.serials || pendingSerials);
          setPayState("success");
          return;
        }
        if (status === "failed") {
          setErrMsg("Payment was declined. Please try again.");
          setPayState("momo_input");
          return;
        }
      } catch { /* keep polling */ }

      if (attempts < maxAttempts) {
        setTimeout(poll, 5000);
      } else {
        setErrMsg("Payment timed out. If you approved the MoMo prompt, your tickets will appear in My Tickets shortly.");
        setPayState("momo_input");
      }
    };

    setTimeout(poll, 5000);
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

  const price      = game.base_price;
  const total      = (qty * price).toFixed(2);
  const maxQty     = game.max_tickets_per_player || 10;
  const hasSchedule = !!schedule;
  const timeLeft   = days > 0
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
              {days > 0 && (
                <div className="bg-card border border-border rounded-lg px-3 py-2 text-center">
                  <div className="font-heading text-xl text-primary">{String(days).padStart(2,"0")}</div>
                  <div className="text-[10px] text-muted-foreground">DAYS</div>
                </div>
              )}
              {[{l:"HRS",v:hours},{l:"MIN",v:minutes},{l:"SEC",v:seconds}].map(t => (
                <div key={t.l} className="bg-card border border-border rounded-lg px-3 py-2 text-center">
                  <div className="font-heading text-xl text-primary">{String(t.v).padStart(2,"0")}</div>
                  <div className="text-[10px] text-muted-foreground">{t.l}</div>
                </div>
              ))}
            </div>
            {game.draw_date && (
              <p className="text-xs text-muted-foreground mb-5">
                Draw: {new Date(game.draw_date).toLocaleDateString("en-GB",{day:"numeric",month:"long",year:"numeric"})} at {game.draw_time}
              </p>
            )}

            {/* Purchase card */}
            <div className="bg-card border border-border rounded-xl p-6 mt-auto">

              {/* ── SUCCESS ── */}
              {payState === "success" && (
                <div className="text-center">
                  <CheckCircle2 className="text-green-400 mx-auto mb-3" size={48} />
                  <h3 className="font-heading text-xl text-foreground mb-2">You're in! Good Luck 🎉</h3>
                  <p className="text-muted-foreground text-sm mb-4">{tickets.length} ticket{tickets.length > 1 ? "s" : ""} confirmed.</p>
                  <div className="flex flex-wrap gap-2 justify-center mb-4">
                    {tickets.map(t => (
                      <span key={t} className="bg-primary/10 text-primary border border-primary/20 px-3 py-1 rounded-md text-sm font-mono">{t}</span>
                    ))}
                  </div>
                  <p className="text-xs text-muted-foreground mb-4">Draw in {timeLeft}</p>
                  <Link to="/my-tickets" className="text-primary text-sm hover:underline">View My Tickets →</Link>
                </div>
              )}

              {/* ── WAITING FOR MOMO PIN ── */}
              {payState === "waiting" && (
                <div className="text-center py-4">
                  <Loader2 className="animate-spin text-primary mx-auto mb-4" size={40} />
                  <h3 className="font-heading text-lg text-foreground mb-2">Waiting for payment...</h3>
                  <p className="text-muted-foreground text-sm mb-1">A MoMo prompt has been sent to</p>
                  <p className="text-primary font-mono font-bold text-lg">{momoPhone}</p>
                  <p className="text-muted-foreground text-xs mt-3">Enter your MoMo PIN on your phone to confirm.</p>
                  {txRef && <p className="text-muted-foreground text-xs mt-1">Ref: {txRef}</p>}
                </div>
              )}

              {/* ── MOMO PHONE INPUT ── */}
              {payState === "momo_input" && (
                <div className="space-y-4">
                  <div className="flex items-center gap-2">
                    <Smartphone size={18} className="text-primary" />
                    <h3 className="font-heading text-base text-foreground">Mobile Money Payment</h3>
                  </div>

                  {errMsg && (
                    <div className="flex items-center gap-2 text-red-400 text-xs bg-red-400/10 rounded-lg px-3 py-2">
                      <AlertCircle size={13} /> {errMsg}
                    </div>
                  )}

                  {/* Network selector */}
                  <div>
                    <p className="text-xs text-muted-foreground mb-2">Select network</p>
                    <div className="grid grid-cols-3 gap-2">
                      {NETWORKS.map(n => (
                        <button key={n.id} onClick={() => setMomoNetwork(n.id)}
                          className={`py-2 px-3 rounded-lg border text-xs font-semibold transition ${momoNetwork === n.id ? "border-primary bg-primary/10 text-primary" : "border-border text-muted-foreground hover:border-primary/40"}`}>
                          {n.label}
                        </button>
                      ))}
                    </div>
                  </div>

                  {/* Phone input */}
                  <div>
                    <p className="text-xs text-muted-foreground mb-1">MoMo phone number</p>
                    <div className="relative">
                      <Phone className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
                      <input type="tel" value={momoPhone}
                        onChange={e => setMomoPhone(e.target.value.replace(/\s+/g, ""))}
                        placeholder="+233XXXXXXXXX"
                        className="w-full pl-10 pr-4 py-2 bg-background border border-input rounded-lg text-foreground text-sm focus:outline-none focus:ring-2 focus:ring-ring" />
                    </div>
                  </div>

                  <div className="flex items-center justify-between text-sm">
                    <span className="text-muted-foreground">Total ({qty} ticket{qty > 1 ? "s" : ""})</span>
                    <span className="text-primary font-bold text-lg">GHS {total}</span>
                  </div>

                  <div className="flex gap-3">
                    <button onClick={() => { setPayState("idle"); setErrMsg(""); }}
                      className="flex-1 border border-border text-muted-foreground py-3 rounded-lg text-sm hover:border-primary/50 hover:text-foreground transition">
                      Back
                    </button>
                    <button onClick={handleInitiatePayment}
                      className="flex-1 bg-primary text-white font-heading py-3 rounded-lg hover:brightness-110 transition flex items-center justify-center gap-2">
                      <Smartphone size={16} /> Pay GHS {total}
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
                    <span className="flex items-center justify-center gap-1 text-xs mt-0.5">
                      <Smartphone size={11} /> Pay via MoMo
                    </span>
                  </div>

                  {!hasSchedule && (
                    <div className="text-xs text-yellow-400 bg-yellow-400/10 rounded-lg px-3 py-2 mb-3 text-center">
                      Ticket sales not yet open for this competition
                    </div>
                  )}

                  <button
                    onClick={() => { if (!isLoggedIn) { navigate("/sign-in"); return; } setPayState("momo_input"); }}
                    disabled={!hasSchedule}
                    className="w-full bg-primary text-white font-heading text-lg py-4 rounded-lg btn-glow hover:brightness-110 transition disabled:opacity-60"
                  >
                    {!isLoggedIn ? "SIGN IN TO BUY" : "BUY WITH MOBILE MONEY"}
                  </button>

                  {!isLoggedIn && (
                    <p className="text-center text-xs text-muted-foreground mt-2">
                      <Link to="/sign-in" className="text-primary hover:underline">Sign in</Link> or{" "}
                      <Link to="/sign-up" className="text-primary hover:underline">create account</Link>
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
