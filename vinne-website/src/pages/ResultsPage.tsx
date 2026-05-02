import { useEffect, useState } from "react";
import Navbar from "@/components/Navbar";
import Footer from "@/components/Footer";
import { Trophy, Calendar, Ticket, AlertCircle, Tv2 } from "lucide-react";

interface Winner {
  id?: string;
  player_name?: string;
  phone_number?: string;
  ticket_serial?: string;
  game_name?: string;
  game_code?: string;
  prize_amount?: number;
  prize_description?: string;
  draw_date?: string;
  position?: number;
}

interface CompletedDraw {
  id: string;
  game_name: string;
  game_code: string;
  draw_date: string;
  winning_numbers?: number[];
  winners?: Winner[];
  total_winners?: number;
  prize_pool?: number;
}

const maskPhone = (phone: string) => {
  if (!phone || phone.length < 6) return "****";
  return phone.slice(0, 3) + "****" + phone.slice(-3);
};

const maskName = (name: string) => {
  if (!name) return "Anonymous";
  const parts = name.trim().split(" ");
  return parts[0] + (parts[1] ? " " + parts[1][0] + "." : "");
};

const ResultsPage = () => {
  const [draws, setDraws] = useState<CompletedDraw[]>([]);
  const [winners, setWinners] = useState<Winner[]>([]);
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState<"draws" | "winners">("winners");

  useEffect(() => {
    const base = import.meta.env.VITE_API_URL || "/api/v1";
    Promise.all([
      fetch(`${base}/public/winners?limit=20`).then(r => r.json()),
      fetch(`${base}/public/draws/completed`).then(r => r.json()),
    ])
      .then(([wData, dData]) => {
        const rawWinners = wData.winners || wData.data?.winners || [];
        // Normalise API field names to interface fields
        setWinners(rawWinners.map((w: Record<string, any>) => ({
          id: w.id || w.ticket_id,
          player_name: w.player_name || w.name || '',
          phone_number: w.phone_number || w.phone || '',
          ticket_serial: w.ticket_serial || w.serial_number || '',
          game_name: w.game_name || w.prize || '',
          game_code: w.game_code || '',
          prize_amount: w.prize_amount || w.winning_amount || 0,
          prize_description: w.prize_description || w.prize || '',
          draw_date: w.draw_date || w.won_at || '',
          position: w.position || 1,
        })));
        setDraws(dData.draws || dData.data?.draws || []);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  return (
    <div className="min-h-screen flex flex-col bg-background">
      <Navbar />
      <main className="flex-1 container pt-24 pb-16">
        {/* Header */}
        <div className="flex items-center gap-3 mb-8">
          <div className="w-10 h-10 rounded-xl bg-primary/10 border border-primary/30 flex items-center justify-center">
            <Trophy size={20} className="text-primary" />
          </div>
          <div>
            <h1 className="font-heading text-2xl text-foreground tracking-wide">RESULTS</h1>
            <p className="text-sm text-muted-foreground">Real winners from our competitions</p>
          </div>
        </div>

        {/* Tabs */}
        <div className="flex gap-2 mb-8">
          {(["winners", "draws"] as const).map(t => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={`px-5 py-2 rounded-full text-sm font-semibold transition border ${
                tab === t ? "bg-primary text-white border-primary" : "border-border text-muted-foreground hover:border-primary/50 hover:text-foreground"
              }`}
            >
              {t === "winners" ? "🏆 Winners" : "📋 Draw Results"}
            </button>
          ))}
        </div>

        {loading ? (
          <div className="space-y-3">
            {[...Array(5)].map((_, i) => (
              <div key={i} className="bg-card border border-border rounded-xl h-20 animate-pulse" />
            ))}
          </div>
        ) : tab === "winners" ? (
          winners.length === 0 ? (
            <div className="text-center py-20">
              <Trophy size={48} className="text-muted-foreground/30 mx-auto mb-4" />
              <h3 className="font-heading text-xl text-foreground mb-2">No winners yet</h3>
              <p className="text-muted-foreground text-sm">Winners will appear here after draws are completed</p>
            </div>
          ) : (
            <div className="space-y-3">
              {/* Live Reveal button — opens the fullscreen draw reveal page */}
              <button
                onClick={() => window.open("/draw-reveal", "_blank")}
                className="w-full flex items-center justify-center gap-3 py-4 rounded-xl font-bold text-base tracking-wide transition-all active:scale-95"
                style={{
                  background: "linear-gradient(135deg, #fde047, #f59e0b)",
                  color: "#000",
                  boxShadow: "0 0 24px rgba(253,224,71,0.35)",
                }}
              >
                <Tv2 size={20} />
                Open Live Draw Reveal Screen
              </button>

              {winners.map((w, i) => (
                <div key={w.id || i} className="bg-card border border-border rounded-xl px-5 py-4 flex items-center gap-4 hover:border-primary/30 transition">
                  {/* Position badge */}
                  <div className={`w-9 h-9 rounded-full flex items-center justify-center font-heading text-sm shrink-0 ${
                    w.position === 1 ? "bg-yellow-500/20 text-yellow-400 border border-yellow-500/30" :
                    w.position === 2 ? "bg-gray-400/20 text-gray-300 border border-gray-400/30" :
                    w.position === 3 ? "bg-orange-500/20 text-orange-400 border border-orange-500/30" :
                    "bg-secondary text-muted-foreground border border-border"
                  }`}>
                    {w.position === 1 ? "🥇" : w.position === 2 ? "🥈" : w.position === 3 ? "🥉" : `#${w.position || i + 1}`}
                  </div>

                  <div className="flex-1 min-w-0">
                    <p className="font-semibold text-foreground text-sm">
                      {w.player_name ? maskName(w.player_name) : w.phone_number ? maskPhone(w.phone_number) : "Lucky Winner"}
                    </p>
                    <p className="text-xs text-muted-foreground truncate">{w.game_name || w.game_code}</p>
                  </div>

                  <div className="text-right shrink-0">
                    {w.prize_description ? (
                      <p className="text-sm font-bold text-primary">{w.prize_description}</p>
                    ) : w.prize_amount ? (
                      <p className="text-sm font-bold text-green-400">₵{w.prize_amount.toLocaleString()}</p>
                    ) : null}
                    {w.draw_date && (
                      <p className="text-xs text-muted-foreground mt-0.5">
                        {new Date(w.draw_date).toLocaleDateString("en-GB", { day: "numeric", month: "short", year: "numeric" })}
                      </p>
                    )}
                    {w.ticket_serial && (
                      <p className="text-xs text-muted-foreground font-mono">{w.ticket_serial}</p>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )
        ) : (
          draws.length === 0 ? (
            <div className="text-center py-20">
              <AlertCircle size={48} className="text-muted-foreground/30 mx-auto mb-4" />
              <h3 className="font-heading text-xl text-foreground mb-2">No completed draws yet</h3>
              <p className="text-muted-foreground text-sm">Draw results will appear here once competitions close</p>
            </div>
          ) : (
            <div className="space-y-4">
              {draws.map(d => (
                <div key={d.id} className="bg-card border border-border rounded-xl overflow-hidden hover:border-primary/30 transition">
                  <div className="bg-secondary px-5 py-3 flex items-center justify-between">
                    <div>
                      <p className="font-heading text-sm text-foreground">{d.game_name || d.game_code}</p>
                      {d.draw_date && (
                        <p className="text-xs text-muted-foreground flex items-center gap-1 mt-0.5">
                          <Calendar size={11} /> {new Date(d.draw_date).toLocaleDateString("en-GB", { day: "numeric", month: "long", year: "numeric" })}
                        </p>
                      )}
                    </div>
                    <div className="flex items-center gap-3">
                      {(d.total_winners || d.winners?.length || 0) > 0 && (
                        <button
                          onClick={() => window.open(`/draw-reveal?drawId=${d.id}`, "_blank")}
                          className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-bold tracking-wide transition-all active:scale-95"
                          style={{ background: "linear-gradient(135deg, #fde047, #f59e0b)", color: "#000" }}
                        >
                          <Tv2 size={13} />
                          Reveal
                        </button>
                      )}
                      <div className="text-right">
                        <p className="text-xs text-muted-foreground flex items-center gap-1 justify-end">
                          <Ticket size={11} /> {d.total_winners || d.winners?.length || 0} winner{(d.total_winners || d.winners?.length || 0) !== 1 ? "s" : ""}
                        </p>
                      </div>
                    </div>
                  </div>

                  {d.winning_numbers && d.winning_numbers.length > 0 && (
                    <div className="px-5 py-3 border-b border-border">
                      <p className="text-xs text-muted-foreground mb-2">Winning Numbers</p>
                      <div className="flex flex-wrap gap-2">
                        {d.winning_numbers.map((n, i) => (
                          <span key={i} className="w-9 h-9 flex items-center justify-center rounded-full bg-primary text-white text-sm font-bold">
                            {n}
                          </span>
                        ))}
                      </div>
                    </div>
                  )}

                  {d.winners && d.winners.length > 0 && (
                    <div className="px-5 py-3 space-y-2">
                      {d.winners.slice(0, 3).map((w, i) => (
                        <div key={i} className="flex items-center justify-between text-sm">
                          <span className="text-foreground/80">
                            {w.player_name ? maskName(w.player_name) : w.phone_number ? maskPhone(w.phone_number) : `Winner ${i + 1}`}
                          </span>
                          <span className="text-green-400 font-semibold">
                            {w.prize_description || (w.prize_amount ? `₵${w.prize_amount.toLocaleString()}` : "Prize")}
                          </span>
                        </div>
                      ))}
                      {d.winners.length > 3 && (
                        <p className="text-xs text-muted-foreground">+{d.winners.length - 3} more winners</p>
                      )}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )
        )}
      </main>
      <Footer />
    </div>
  );
};

export default ResultsPage;
