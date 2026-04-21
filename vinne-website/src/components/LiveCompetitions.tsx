import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Trophy, Loader2 } from "lucide-react";
import { fetchActiveGames, type ApiGame } from "@/lib/api";
import { useCountdown } from "@/hooks/useCountdown";

const BASE = import.meta.env.VITE_API_URL || "/api/v1";

const getNextDrawDate = (game: ApiGame): Date => {
  if (game.draw_date) return new Date(game.draw_date + "T" + (game.draw_time || "20:00") + ":00Z");
  const [h, m] = (game.draw_time || "20:00").split(":").map(Number);
  const now = new Date();
  const next = new Date(now);
  next.setUTCHours(h, m, 0, 0);
  if (next <= now) next.setUTCDate(next.getUTCDate() + 1);
  return next;
};

const getPrize = (game: ApiGame) => {
  try { const p = JSON.parse(game.prize_details || "[]"); return p[0]?.description || ""; } catch { return ""; }
};

const getEndsLabel = (days: number, drawDate: Date) => {
  if (days === 0) return "ENDS TODAY";
  if (days === 1) return "ENDS TOMORROW";
  if (days <= 7) return `ENDS IN ${days} DAYS`;
  return `ENDS ${new Date(drawDate).toLocaleDateString("en-GB", { weekday: "short", day: "numeric", month: "short" }).toUpperCase()}`;
};

const useTicketsSold = (gameId: string) => {
  const [sold, setSold] = useState(0);
  useEffect(() => {
    fetch(`${BASE}/players/games/${gameId}/schedule`)
      .then(r => r.json())
      .then(d => {
        const s = (d?.data?.schedules ?? []).find((s: { status: string; is_active: boolean }) => s.status === "SCHEDULED" && s.is_active) ?? d?.data?.schedules?.[0];
        if (s?.tickets_sold != null) setSold(s.tickets_sold);
      }).catch(() => {});
  }, [gameId]);
  return sold;
};

// ── Competition Card — dark theme matching hero ───────────────────────────────
const CompCard = ({ game }: { game: ApiGame }) => {
  const drawDate = getNextDrawDate(game);
  const { days } = useCountdown(drawDate);
  const sold = useTicketsSold(game.id);
  const total = game.total_tickets || 1000;
  const pct = Math.min(100, Math.round((sold / total) * 100));
  const prize = getPrize(game);
  const endsLabel = getEndsLabel(days, drawDate);
  const isUrgent = days <= 1;

  return (
    <div className="rounded-2xl overflow-hidden flex flex-col"
      style={{ background: "hsl(0 0% 8%)", border: "1px solid hsl(0 0% 14%)" }}>

      {/* Image */}
      <div className="relative aspect-[4/3] overflow-hidden bg-black">
        {game.logo_url
          ? <img src={`${game.logo_url}?t=${Math.floor(Date.now() / 3600000)}`} alt={game.name}
              className="w-full h-full object-cover hover:scale-105 transition-transform duration-500" />
          : <div className="w-full h-full flex items-center justify-center"><Trophy size={64} className="text-white/10" /></div>
        }
        {/* ENDS badge */}
        <span className="absolute top-3 left-3 font-bold text-white px-3 py-1 rounded-lg shadow-lg"
          style={{ background: "linear-gradient(90deg,#ff0080,#ff6000)", fontFamily: "'Poppins',sans-serif", fontSize: "0.72rem" }}>
          {endsLabel}
        </span>
        {/* Red prize banner */}
        <div className="absolute bottom-0 left-0 right-0 py-2.5 px-4 text-center"
          style={{ background: isUrgent ? "#8B0000" : "#cc0000" }}>
          <p className="font-bold text-white text-sm tracking-wide uppercase"
            style={{ fontFamily: "'Poppins',sans-serif" }}>
            {prize ? `${prize}!` : game.name.toUpperCase()}
          </p>
        </div>
      </div>

      {/* Card body — dark */}
      <div className="p-5 flex flex-col flex-1">
        {/* Description */}
        <p className="text-white/60 text-sm text-center leading-snug mb-4 flex-1"
          style={{ fontFamily: "'Poppins',sans-serif", fontWeight: 500 }}>
          {game.description || `Win a ${prize || game.name}!`}
        </p>

        {/* Ticket price */}
        <div className="text-center mb-4">
          <p className="text-white/40 text-xs font-semibold tracking-widest uppercase mb-0.5"
            style={{ fontFamily: "'Poppins',sans-serif" }}>TICKET PRICE</p>
          <p className="font-heading font-black text-gold text-3xl">GHS {game.base_price.toFixed(2)}</p>
        </div>

        {/* Progress */}
        <div className="mb-4">
          <div className="flex items-center justify-between mb-1.5">
            <span className="font-bold text-xs" style={{ color: "hsl(22 100% 52%)", fontFamily: "'Poppins',sans-serif" }}>
              Sold {pct}%
            </span>
            <span className="text-white/30 text-xs" style={{ fontFamily: "'Poppins',sans-serif" }}>
              {(total - sold).toLocaleString()} Left
            </span>
          </div>
          <div className="h-1.5 rounded-full overflow-hidden" style={{ background: "hsl(0 0% 18%)" }}>
            <div className="h-full rounded-full transition-all duration-700"
              style={{ width: `${Math.max(pct, 2)}%`, background: "hsl(22 100% 52%)" }} />
          </div>
        </div>

        {/* Button */}
        <Link to={`/competitions/${game.id}`}
          className="w-full font-heading font-black text-base py-3 rounded-xl text-center transition tracking-widest"
          style={{
            border: "1.5px solid hsl(44 100% 52% / 0.7)",
            color: "white",
            background: "transparent",
            boxShadow: "0 0 12px hsl(44 100% 52% / 0.2)",
          }}
          onMouseEnter={e => (e.currentTarget.style.boxShadow = "0 0 24px hsl(44 100% 52% / 0.5)")}
          onMouseLeave={e => (e.currentTarget.style.boxShadow = "0 0 12px hsl(44 100% 52% / 0.2)")}
        >
          ENTER NOW »
        </Link>
      </div>
    </div>
  );
};

// ── Section ───────────────────────────────────────────────────────────────────
const LiveCompetitions = () => {
  const [games, setGames] = useState<ApiGame[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchActiveGames().then(setGames).catch(console.error).finally(() => setLoading(false));
  }, []);

  return (
    <section className="py-12 bg-[hsl(0_0%_4%)]">
      <div className="container">
        <h2 className="font-heading font-black text-gold text-3xl md:text-4xl mb-8 tracking-wide">
          LIVE COMPETITIONS
        </h2>

        {loading ? (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="animate-spin text-primary" size={32} />
          </div>
        ) : games.length === 0 ? (
          <p className="text-muted-foreground text-center py-8">No active competitions right now.</p>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6">
            {games.map(g => <CompCard key={g.id} game={g} />)}
          </div>
        )}

        {games.length > 0 && (
          <div className="mt-8 text-center">
            <Link to="/competitions"
              className="inline-flex items-center gap-2 font-heading font-black text-sm px-8 py-3 rounded-xl transition tracking-wide text-white/60 hover:text-white"
              style={{ border: "1px solid hsl(0 0% 20%)" }}>
              VIEW ALL COMPETITIONS →
            </Link>
          </div>
        )}
      </div>
    </section>
  );
};

export default LiveCompetitions;
