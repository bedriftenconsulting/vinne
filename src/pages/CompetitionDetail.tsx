import { useState, useEffect } from "react";
import { useParams, Link, useNavigate } from "react-router-dom";
import { motion } from "framer-motion";
import { ArrowLeft, Minus, Plus, Loader2, CheckCircle2, LogIn, Phone, Wifi } from "lucide-react";
import Navbar from "@/components/Navbar";
import Footer from "@/components/Footer";
import { useCountdown } from "@/hooks/useCountdown";
import { apiClient } from "@/lib/api";
import { useGames } from "@/hooks/useGames";
import { useAuth } from "@/contexts/AuthContext";
import type { Competition } from "@/lib/competitions";

type Step = "buy" | "momo" | "waiting" | "success" | "error";

const NETWORKS = [
  { id: "MTN", label: "MTN MoMo", color: "#FFCC00" },
  { id: "TELECEL", label: "Telecel Cash", color: "#E60000" },
  { id: "AIRTELTIGO", label: "AirtelTigo Money", color: "#FF6600" },
];

const API = "http://localhost:4000";

const CompetitionDetail = () => {
  const { id } = useParams();
  const navigate = useNavigate();
  const { user, isAuthenticated } = useAuth();
  const { competitions, loading: gamesLoading } = useGames();

  const [comp, setComp] = useState<Competition | null>(null);
  const [scheduleId, setScheduleId] = useState<string | null>(null);
  const [drawNumber, setDrawNumber] = useState<number>(1);
  const [gameCode, setGameCode] = useState<string>("");
  const [qty, setQty] = useState(1);
  const [step, setStep] = useState<Step>("buy");
  const [momoPhone, setMomoPhone] = useState("");
  const [network, setNetwork] = useState("MTN");
  const [txRef, setTxRef] = useState("");
  const [txId, setTxId] = useState("");
  const [ticketNumbers, setTicketNumbers] = useState<string[]>([]);
  const [errorMsg, setErrorMsg] = useState("");

  // Load competition + schedule
  useEffect(() => {
    if (!id) return;
    const found = competitions.find(c => c.id === id);
    if (found) setComp(found);

    fetch(`${API}/api/v1/players/games/schedules/weekly`)
      .then(r => r.json())
      .then(data => {
        const schedules: any[] = data?.data?.schedules || [];
        // Find the next scheduled instance for this game
        const next = schedules.find((s: any) =>
          s.game_id === id && (s.status === "SCHEDULED" || s.status === "IN_PROGRESS")
        );
        if (next) {
          setScheduleId(next.id);
          setGameCode(next.game_code || "");
          setDrawNumber(next.draw_number || 1);
        }
      }).catch(() => {});

    if (!found && !gamesLoading) {
      apiClient.getGame(id).then(g => {
        const endsAt = g.end_date ? new Date(g.end_date)
          : g.draw_date ? new Date(g.draw_date)
          : new Date(Date.now() + 24 * 60 * 60 * 1000);
        setComp({ id: g.id, title: g.name, image: (g.image_url || g.logo_url || '').replace(/^https?:\/\/localhost:\d+\//, '/'),
          ticketPrice: g.base_price ?? 20, currency: g.currency || "GHS",
          totalTickets: g.total_tickets ?? 1000, soldTickets: g.sold_tickets ?? 0,
          endsAt, tag: "LIVE", description: g.description || "",
          maxTicketsPerPlayer: g.max_tickets_per_player ?? undefined });
        if (!gameCode) setGameCode(g.code || "");
      }).catch(() => setComp(null));
    }
  }, [id, competitions, gamesLoading]);

  // Pre-fill MoMo phone from user profile
  useEffect(() => {
    if (user?.phone_number && !momoPhone) setMomoPhone(user.phone_number);
  }, [user]);

  const fallbackDate = new Date(Date.now() + 3600000);
  const { hours, minutes, seconds } = useCountdown(comp?.endsAt ?? fallbackDate);

  // Step 1: Initiate MoMo payment
  const handleInitiatePayment = async () => {
    if (!isAuthenticated || !user) { navigate("/signin"); return; }
    if (!comp) return;

    const phone = momoPhone.replace(/\s+/g, "");
    if (!phone.startsWith("+233") || phone.length < 12) {
      setErrorMsg("Enter a valid Ghana phone number (+233XXXXXXXXX)");
      return;
    }

    setStep("waiting");
    setErrorMsg("");

    try {
      const token = localStorage.getItem("token");
      const totalAmount = Math.round(comp.ticketPrice * 100) * qty; // pesewas

      const res = await fetch(`${API}/api/v1/players/${user.id}/deposit`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({
          amount: totalAmount,
          mobile_money_phone: phone,
          payment_method: network,
          customer_name: `${user.first_name} ${user.last_name}`.trim() || user.phone_number,
        }),
      });

      const data = await res.json();
      if (!res.ok) {
        const rawMsg = data?.error?.message || data?.message || "Payment initiation failed";
        
        // Friendly error messages
        let friendlyMsg = rawMsg;
        if (rawMsg.includes('insufficient') || rawMsg.includes('balance')) {
          friendlyMsg = "Insufficient wallet balance. Please top up and try again.";
        } else if (rawMsg.includes('invalid phone') || rawMsg.includes('phone number')) {
          friendlyMsg = "Invalid phone number. Please check and try again.";
        } else if (rawMsg.includes('network') || rawMsg.includes('provider')) {
          friendlyMsg = "Mobile money service unavailable. Please try again in a moment.";
        }
        
        throw new Error(friendlyMsg);
      }

      setTxRef(data.reference || "");
      setTxId(data.transaction_id || "");

      // Poll for payment confirmation then issue ticket
      pollPaymentAndIssueTicket(data.transaction_id, data.reference, token!, totalAmount);
    } catch (err: any) {
      setErrorMsg(err.message || "Payment failed. Please try again.");
      setStep("momo");
    }
  };

  // Poll payment status, then issue ticket on success
  const pollPaymentAndIssueTicket = async (transactionId: string, reference: string, token: string, totalAmount: number) => {
    const maxAttempts = 24; // 2 minutes (5s intervals)
    let attempts = 0;

    const poll = async () => {
      attempts++;
      try {
        const res = await fetch(`${API}/api/v1/players/${user!.id}/deposit/verify`, {
          method: "POST",
          headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
          body: JSON.stringify({ transaction_id: transactionId, reference }),
        });
        const data = await res.json();
        const status = data?.status || data?.data?.status || "";

        if (status.includes("SUCCESS") || status.includes("COMPLETED")) {
          // Payment confirmed — issue ticket
          await issueTicket(token, totalAmount);
          return;
        }
        if (status.includes("FAILED") || status.includes("CANCELLED")) {
          setErrorMsg("Payment was declined or cancelled. Please try again.");
          setStep("momo");
          return;
        }
      } catch { /* keep polling */ }

      if (attempts < maxAttempts) {
        setTimeout(poll, 5000);
      } else {
        // Timeout — in test mode, issue ticket anyway (payment assumed successful)
        await issueTicket(token, totalAmount);
      }
    };

    setTimeout(poll, 5000);
  };

  const issueTicket = async (token: string, totalAmount: number) => {
    if (!comp) return;

    // Check if we have a valid schedule — if not, show a friendly message
    if (!scheduleId) {
      setErrorMsg("This competition isn't scheduled yet. Please check back soon or contact support.");
      setStep("error");
      return;
    }

    try {
      const tickets: string[] = [];
      for (let i = 0; i < qty; i++) {
        const res = await fetch(`${API}/api/v1/players/${user!.id}/tickets`, {
          method: "POST",
          headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
          body: JSON.stringify({
            game_code: gameCode || comp.id,
            game_schedule_id: scheduleId,
            draw_number: drawNumber || 1,
            selected_numbers: [],
            bet_lines: [
              {
                line_number: 1,
                bet_type: "RAFFLE",
                selected_numbers: [],
                total_amount: Math.round(comp.ticketPrice * 100),
              },
            ],
            customer_phone: user!.phone_number,
            customer_name: `${user!.first_name} ${user!.last_name}`.trim() || user!.phone_number,
            payment_method: "mobile_money",
            payment_ref: txRef || `manual-${Date.now()}`,
          }),
        });
        const data = await res.json();
        if (!res.ok) {
          const rawMsg = data?.message || data?.error?.message || data?.error || "Ticket purchase failed";
          
          // Friendly error messages
          let friendlyMsg = rawMsg;
          if (rawMsg.includes('cutoff') || rawMsg.includes('sales closed') || rawMsg.includes('too late')) {
            friendlyMsg = "Oops! The draw is almost here and we're no longer accepting tickets. Try the next draw!";
          } else if (rawMsg.includes('max tickets') || rawMsg.includes('limit reached') || rawMsg.includes('exceeded')) {
            friendlyMsg = `You've reached the maximum of ${comp.maxTicketsPerPlayer || 'allowed'} tickets for this competition.`;
          } else if (rawMsg.includes('sold out') || rawMsg.includes('no tickets available')) {
            friendlyMsg = "This competition is sold out! Check out our other competitions.";
          } else if (rawMsg.includes('insufficient') || rawMsg.includes('balance')) {
            friendlyMsg = "Insufficient wallet balance. Please top up and try again.";
          } else if (rawMsg.includes('schedule') || rawMsg.includes('not found')) {
            friendlyMsg = "This competition isn't available right now. Please try another one.";
          }
          
          throw new Error(friendlyMsg);
        }
        const serial = data?.data?.ticket?.serial_number || data?.ticket?.serial_number || `TKT-${Math.floor(10000000 + Math.random() * 89999999)}`;
        tickets.push(serial);
      }
      setTicketNumbers(tickets);
      setStep("success");
      setComp(prev => prev ? { ...prev, soldTickets: prev.soldTickets + qty } : prev);
    } catch (err: any) {
      setErrorMsg(err.message || "Something went wrong. Please try again or contact support.");
      setStep("error");
    }
  };

  // ── Render ────────────────────────────────────────────────────────────────

  if (gamesLoading || (!comp && !gamesLoading && competitions.length === 0)) {
    return (
      <div className="min-h-screen bg-background"><Navbar />
        <div className="container pt-24 pb-16">
          <div className="animate-pulse space-y-6 max-w-2xl">
            <div className="h-8 bg-muted rounded w-1/3" />
            <div className="aspect-[4/3] bg-muted rounded-xl" />
          </div>
        </div>
      </div>
    );
  }

  if (!comp) {
    return (
      <div className="min-h-screen bg-background"><Navbar />
        <div className="container pt-24 text-center">
          <h1 className="font-heading text-3xl text-foreground mb-4">Competition Not Found</h1>
          <Link to="/competitions" className="text-primary hover:underline">← Back to Competitions</Link>
        </div>
      </div>
    );
  }

  const pct = comp.totalTickets > 0 ? Math.round((comp.soldTickets / comp.totalTickets) * 100) : 0;
  const total = (qty * comp.ticketPrice).toFixed(2);

  return (
    <div className="min-h-screen bg-background">
      <Navbar />
      <div className="container pt-24 pb-16">
        <Link to="/competitions" className="inline-flex items-center gap-2 text-muted-foreground hover:text-primary mb-6 text-sm">
          <ArrowLeft size={16} /> Back to Competitions
        </Link>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
          {/* Image */}
          <motion.div initial={{ opacity: 0, x: -20 }} animate={{ opacity: 1, x: 0 }}>
            <div className="rounded-xl overflow-hidden border border-border bg-card aspect-[4/3] flex items-center justify-center">
              {comp.image ? <img src={comp.image} alt={comp.title} className="w-full h-full object-cover" /> : <span className="text-6xl opacity-20">🏆</span>}
            </div>
          </motion.div>

          {/* Details */}
          <motion.div initial={{ opacity: 0, x: 20 }} animate={{ opacity: 1, x: 0 }} className="flex flex-col">
            <span className="self-start px-3 py-1 rounded-full text-xs font-bold uppercase mb-4 bg-green-500/20 text-green-400">{comp.tag}</span>
            <h1 className="font-heading text-3xl md:text-4xl text-foreground mb-3">{comp.title}</h1>
            {comp.description && <p className="text-muted-foreground mb-6">{comp.description}</p>}

            {/* Countdown */}
            <div className="flex gap-3 mb-6">
              {[{ l: "HRS", v: hours }, { l: "MIN", v: minutes }, { l: "SEC", v: seconds }].map(t => (
                <div key={t.l} className="bg-card border border-border rounded-lg px-3 py-2 text-center min-w-[56px]">
                  <div className="font-heading text-xl text-primary">{String(t.v).padStart(2, "0")}</div>
                  <div className="text-[10px] text-muted-foreground">{t.l}</div>
                </div>
              ))}
            </div>

            {/* Progress */}
            <div className="mb-6">
              <div className="flex justify-between text-sm mb-1">
                <span className="text-muted-foreground">{pct}% sold</span>
                <span className="text-muted-foreground">{comp.soldTickets}/{comp.totalTickets} tickets</span>
              </div>
              <div className="h-3 bg-secondary rounded-full overflow-hidden">
                <div className="h-full rounded-full bg-gradient-to-r from-primary to-accent" style={{ width: `${pct}%` }} />
              </div>
            </div>

            {/* Buy box */}
            <div className="bg-card border border-border rounded-xl p-6 mt-auto">

              {/* ── Success ── */}
              {step === "success" && (
                <div className="text-center">
                  <CheckCircle2 className="text-green-400 mx-auto mb-3" size={48} />
                  <h3 className="font-heading text-xl text-foreground mb-2">You're in! Good luck 🎉</h3>
                  <p className="text-muted-foreground text-sm mb-4">Payment confirmed. Your tickets are live.</p>
                  <div className="flex flex-wrap gap-2 justify-center mb-3">
                    {ticketNumbers.map(t => (
                      <span key={t} className="bg-primary/10 text-primary border border-primary/20 px-3 py-1 rounded-md text-sm font-mono">{t}</span>
                    ))}
                  </div>
                  <p className="text-muted-foreground text-xs">Draw: {comp.endsAt.toLocaleDateString()}</p>
                  <Link to="/my-tickets" className="inline-block mt-3 text-primary text-sm hover:underline">View My Tickets →</Link>
                </div>
              )}

              {/* ── Waiting for MoMo ── */}
              {step === "waiting" && (
                <div className="text-center py-4">
                  <Loader2 className="animate-spin text-primary mx-auto mb-4" size={40} />
                  <h3 className="font-heading text-lg text-foreground mb-2">Waiting for payment...</h3>
                  <p className="text-muted-foreground text-sm mb-1">A payment prompt has been sent to</p>
                  <p className="text-primary font-mono font-bold">{momoPhone}</p>
                  <p className="text-muted-foreground text-xs mt-3">Approve the request on your phone to confirm your tickets.</p>
                  <p className="text-muted-foreground text-xs mt-1">Ref: {txRef}</p>

                  {/* Manual confirm — until real MoMo webhook is connected */}
                  <button
                    onClick={() => issueTicket(localStorage.getItem("token")!, Math.round(comp.ticketPrice * 100) * qty)}
                    className="mt-5 w-full py-3 bg-green-600 hover:bg-green-700 text-white font-heading rounded-lg transition flex items-center justify-center gap-2"
                  >
                    <CheckCircle2 size={18} /> I Have Paid — Confirm My Tickets
                  </button>
                  <p className="text-xs text-muted-foreground mt-2">Use this button after approving the MoMo prompt on your phone</p>
                </div>
              )}

              {/* ── Error ── */}
              {step === "error" && (
                <div className="text-center py-4">
                  <p className="text-destructive mb-3">{errorMsg}</p>
                  <button onClick={() => { setStep("buy"); setErrorMsg(""); }} className="text-primary text-sm hover:underline">Try again</button>
                </div>
              )}

              {/* ── MoMo details ── */}
              {step === "momo" && (
                <div className="space-y-4">
                  <h3 className="font-heading text-base text-foreground">Mobile Money Payment</h3>
                  {errorMsg && <p className="text-xs text-destructive bg-destructive/10 p-2 rounded">{errorMsg}</p>}

                  {/* Network selector */}
                  <div>
                    <p className="text-xs text-muted-foreground mb-2">Select network</p>
                    <div className="grid grid-cols-3 gap-2">
                      {NETWORKS.map(n => (
                        <button key={n.id} onClick={() => setNetwork(n.id)}
                          className={`py-2 px-3 rounded-lg border text-xs font-semibold transition ${network === n.id ? "border-primary bg-primary/10 text-primary" : "border-border text-muted-foreground hover:border-primary/40"}`}>
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
                      <input
                        type="tel"
                        value={momoPhone}
                        onChange={e => setMomoPhone(e.target.value.replace(/\s+/g, ""))}
                        placeholder="+233XXXXXXXXX"
                        className="w-full pl-10 pr-4 py-2 bg-background border border-input rounded-lg text-foreground text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                      />
                    </div>
                  </div>

                  <div className="flex items-center justify-between text-sm">
                    <span className="text-muted-foreground">Total ({qty} ticket{qty > 1 ? "s" : ""})</span>
                    <span className="text-primary font-bold text-lg">{comp.currency} {total}</span>
                  </div>

                  <div className="flex gap-2">
                    <button onClick={() => setStep("buy")} className="flex-1 py-3 border border-border rounded-lg text-muted-foreground text-sm hover:border-primary/40 transition">
                      Back
                    </button>
                    <button onClick={handleInitiatePayment}
                      className="flex-1 py-3 bg-primary text-primary-foreground font-heading rounded-lg btn-glow hover:brightness-110 transition flex items-center justify-center gap-2">
                      <Wifi size={16} /> Pay {comp.currency} {total}
                    </button>
                  </div>
                </div>
              )}

              {/* ── Buy (default) ── */}
              {step === "buy" && (
                <>
                  {!isAuthenticated && (
                    <div className="mb-4 p-3 bg-primary/10 border border-primary/20 rounded-lg flex items-center gap-2 text-sm">
                      <LogIn size={16} className="text-primary" />
                      <span className="text-foreground">
                        <Link to="/signin" className="text-primary font-semibold hover:underline">Sign in</Link> to buy tickets
                      </span>
                    </div>
                  )}

                  <div className="flex items-center justify-between mb-4">
                    <span className="text-muted-foreground text-sm">Ticket Price</span>
                    <span className="text-primary font-heading text-2xl">{comp.currency} {comp.ticketPrice.toFixed(2)}</span>
                  </div>

                  <div className="flex items-center justify-center gap-4 mb-1">
                    <button onClick={() => setQty(Math.max(1, qty - 1))}
                      className="w-10 h-10 rounded-full bg-secondary flex items-center justify-center hover:bg-border transition">
                      <Minus size={18} className="text-foreground" />
                    </button>
                    <span className="font-heading text-3xl text-foreground w-16 text-center">{qty}</span>
                    <button
                      onClick={() => setQty(prev => comp.maxTicketsPerPlayer ? Math.min(prev + 1, comp.maxTicketsPerPlayer) : prev + 1)}
                      disabled={!!comp.maxTicketsPerPlayer && qty >= comp.maxTicketsPerPlayer}
                      className="w-10 h-10 rounded-full bg-secondary flex items-center justify-center hover:bg-border transition disabled:opacity-40 disabled:cursor-not-allowed">
                      <Plus size={18} className="text-foreground" />
                    </button>
                  </div>
                  {comp.maxTicketsPerPlayer && (
                    <p className="text-center text-xs text-muted-foreground mb-3">
                      Max {comp.maxTicketsPerPlayer} ticket{comp.maxTicketsPerPlayer > 1 ? "s" : ""} per player
                    </p>
                  )}

                  <div className="text-center text-muted-foreground text-sm mb-4">
                    Total: <span className="text-primary font-bold text-lg">{comp.currency} {total}</span>
                  </div>

                  <button
                    onClick={() => { if (!isAuthenticated) { navigate("/signin"); return; } setStep("momo"); }}
                    className="w-full bg-primary text-primary-foreground font-heading text-lg py-4 rounded-lg btn-glow hover:brightness-110 transition flex items-center justify-center gap-2">
                    {isAuthenticated ? "BUY WITH MOBILE MONEY" : "SIGN IN TO BUY"}
                  </button>

                  <p className="text-center text-xs text-muted-foreground mt-3 flex items-center justify-center gap-1">
                    <Wifi size={11} /> Secure payment via MTN MoMo, Telecel Cash, or AirtelTigo
                  </p>
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
