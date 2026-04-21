import { useEffect, useState } from "react";
import Navbar from "@/components/Navbar";
import Footer from "@/components/Footer";
import { Link } from "react-router-dom";
import { Trophy, Clock, Loader2, Users } from "lucide-react";
import { fetchActiveGames, type ApiGame } from "@/lib/api";
import { useCountdown } from "@/hooks/useCountdown";

const BASE = import.meta.env.VITE_API_URL || "/api/v1";

const GameCard = ({ game, index = 0 }: { game: ApiGame; index?: number }) => {
  const drawDate = game.draw_date
    ? new Date(game.draw_date + "T" + (game.draw_time || "20:00") + ":00Z")
    : (() => {
        const [h, m] = (game.draw_time || "20:00").split(":").map(Number);
        const now = new Date();
        const next = new Date(now);
        next.setUTCHours(h, m, 0, 0);
        if (next <= now) next.setUTCDate(next.getUTCDate() + 1);
        return next;
      })();
  const { days, hours, minutes, seconds } = useCountdown(drawDate);
  const [ticketsSold, setTicketsSold] = useState<number>(0);

  useEffect(() => {
    fetch(`${BASE}/players/games/${game.id}/schedule`)
      .then(r => r.json())
      .then(d => {
        const schedules = d?.data?.schedules ?? [];
        const active = schedules.find((s: { status: string; is_active: boolean }) => s.status === "SCHEDULED" && s.is_active) ?? schedules[0];
        if (active?.tickets_sold != null) setTicketsSold(active.tickets_sold);
        else if (active?.total_tickets_sold != null) setTicketsSold(active.total_tickets_sold);
      })
      .catch(() => {});
  }, [game.id]);

  const timeLabel = days > 0
    ? `${days}d ${String(hours).padStart(2, "0")}h ${String(minutes).padStart(2, "0")}m`
    : `${String(hours).padStart(2, "0")}:${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;

  let prizeLabel = "";
  try {
    const prizes = JSON.parse(game.prize_details || "[]");
    if (prizes[0]?.description) prizeLabel = prizes[0].description;
  } catch { /* ignore */ }

  const totalTickets = game.total_tickets || 1000;
  const pct = totalTickets > 0 ? Math.min(100, Math.round((ticketsSold / totalTickets) * 100)) : 0;
  const isFilling = pct >= 75;

  return (
    <Link
      to={`/competitions/${game.id}`}
      className="group block card-light rounded-xl overflow-hidden shadow-md hover:shadow-xl transition-shadow border border-black/8"
      style={{ animationDelay: `${index * 80}ms` }}
    >
      <div className="relative aspect-[4/3] overflow-hidden bg-black/80 flex items-center justify-center">
        {game.logo_url ? (
          <img src={`${game.logo_url}?t=${Math.floor(Date.now() / 3600000)}`} alt={game.name} className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-500" loading="lazy" />
        ) : (
          <Trophy className="h-16 w-16 text-white/20" />
        )}
        <span className="absolute top-3 left-3 text-white px-3 py-1 rounded-lg text-xs font-bold shadow-lg"
          style={{ background: "linear-gradient(90deg,#ff0080,#ff6000)", fontFamily: "'Poppins', sans-serif", fontSize: "0.72rem" }}>
          {days === 0 ? "ENDS TODAY" : days === 1 ? "ENDS TOMORROW" : days <= 7 ? `ENDS IN ${days} DAYS` : `ENDS ${new Date(drawDate).toLocaleDateString("en-GB",{weekday:"short",day:"numeric",month:"short"}).toUpperCase()}`}
        </span>
      </div>

      <div className="p-4">
        <h3 className="font-heading text-base text-[hsl(0_0%_10%)] mb-0.5 leading-tight">{game.name}</h3>
        {prizeLabel && <p className="text-xs text-muted-foreground mb-2 truncate">🏆 {prizeLabel}</p>}

        <div className="flex items-center justify-between mt-2">
          <span className="font-heading text-lg text-[hsl(0_0%_10%)]">GHS {game.base_price.toFixed(2)}</span>
          <span className="w-8 h-8 rounded-full bg-[hsl(22_100%_52%)] flex items-center justify-center text-white font-bold text-lg shadow">+</span>
        </div>

        {/* Ticket progress bar */}
        <div className="mt-3">
          <div className="flex items-center justify-between mb-1.5">
            <span className="flex items-center gap-1 text-xs text-gray-500">
              <Users size={10} />
              {ticketsSold.toLocaleString()} / {totalTickets.toLocaleString()} tickets
            </span>
            <span className={`text-xs font-bold ${isFilling ? "text-orange-500" : "text-gray-400"}`}>
              {pct}% sold
            </span>
          </div>
          <div className="h-2 bg-gray-200 rounded-full overflow-hidden">
            <div
              className={`h-full rounded-full transition-all duration-700 ${isFilling ? "bg-orange-500" : "bg-[hsl(22_100%_52%)]"}`}
              style={{ width: `${Math.max(pct, 3)}%` }}
            />
          </div>
        </div>

        <div className="mt-2 flex items-center gap-1.5 text-xs text-muted-foreground">
          <Clock size={11} />
          {game.draw_date
            ? `Draw: ${new Date(game.draw_date).toLocaleDateString("en-GB", { day: "numeric", month: "short", year: "numeric" })}`
            : `Daily draw at ${game.draw_time || "20:00"}`}
        </div>
      </div>
    </Link>
  );
};

const CompetitionsPage = () => {
  const [games, setGames] = useState<ApiGame[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchActiveGames().then(setGames).catch(console.error).finally(() => setLoading(false));
  }, []);

  return (
    <div className="min-h-screen bg-background">
      <Navbar />
      <div className="container pt-24 pb-16">
        <h1 className="font-heading text-4xl md:text-5xl text-primary mb-10">ALL COMPETITIONS</h1>

        {loading ? (
          <div className="flex items-center justify-center py-20">
            <Loader2 className="animate-spin text-primary" size={36} />
          </div>
        ) : games.length === 0 ? (
          <div className="text-center py-20">
            <Trophy size={48} className="text-muted-foreground/20 mx-auto mb-4" />
            <p className="text-muted-foreground">No active competitions right now. Check back soon!</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6">
            {games.map((g, i) => <GameCard key={g.id} game={g} index={i} />)}
          </div>
        )}
      </div>
      <Footer />
    </div>
  );
};

export default CompetitionsPage;
