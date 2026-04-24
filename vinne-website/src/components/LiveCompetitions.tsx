import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Trophy, Clock, Loader2, Users } from "lucide-react";
import { fetchActiveGames, type ApiGame } from "@/lib/api";
import { useCountdown } from "@/hooks/useCountdown";

const BASE = import.meta.env.VITE_API_URL || "/api/v1";

// Compute next draw date
const getNextDrawDate = (game: ApiGame): Date => {
  if (game.draw_date) {
    return new Date(game.draw_date + "T" + (game.draw_time || "20:00") + ":00Z");
  }
  const [h, m] = (game.draw_time || "20:00").split(":").map(Number);
  const now = new Date();
  const next = new Date(now);
  next.setUTCHours(h, m, 0, 0);
  if (next <= now) next.setUTCDate(next.getUTCDate() + 1);
  return next;
};

const GameCard = ({ game, index = 0 }: { game: ApiGame; index?: number }) => {
  const drawDate = getNextDrawDate(game);
  const { days, hours, minutes, seconds } = useCountdown(drawDate);
  const [ticketsSold, setTicketsSold] = useState<number>(0);

  // Fetch tickets sold count for this game's active schedule
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
          <img src={game.logo_url} alt={game.name} className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-500" loading="lazy" />
        ) : (
          <Trophy className="h-16 w-16 text-white/20" />
        )}
        <span className="absolute top-3 left-3 bg-[hsl(22_100%_52%)] text-white px-3 py-1 rounded-full text-[11px] font-bold uppercase tracking-wide flex items-center gap-1.5">
          <span className="w-1.5 h-1.5 bg-white rounded-full animate-pulse inline-block" />
          CLOSES IN {timeLabel}
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

const LiveCompetitions = () => {
  const [games, setGames] = useState<ApiGame[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchActiveGames().then(setGames).catch(console.error).finally(() => setLoading(false));
  }, []);

  return (
    <section className="py-16 section-light">
      <div className="container">
        <h2 className="font-heading text-3xl md:text-4xl text-[hsl(0_0%_10%)] mb-8 tracking-wide">
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
            {games.map((g, i) => <GameCard key={g.id} game={g} index={i} />)}
          </div>
        )}
      </div>
    </section>
  );
};

export default LiveCompetitions;
