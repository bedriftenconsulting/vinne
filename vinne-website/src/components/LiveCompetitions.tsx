import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Trophy, Clock, Loader2 } from "lucide-react";
import { fetchActiveGames, type ApiGame } from "@/lib/api";
import { useCountdown } from "@/hooks/useCountdown";

// Compute next draw date — handles daily (no draw_date) and special (has draw_date)
const getNextDrawDate = (game: ApiGame): Date => {
  if (game.draw_date) {
    return new Date(game.draw_date + "T" + (game.draw_time || "20:00") + ":00Z");
  }
  // Daily/weekly — next draw at draw_time today or tomorrow
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

  const timeLabel = days > 0
    ? `${days}d ${String(hours).padStart(2, "0")}h ${String(minutes).padStart(2, "0")}m`
    : `${String(hours).padStart(2, "0")}:${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;

  let prizeLabel = "";
  try {
    const prizes = JSON.parse(game.prize_details || "[]");
    if (prizes[0]?.description) prizeLabel = prizes[0].description;
  } catch { /* ignore */ }

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
        <div className="mt-3 flex items-center gap-1.5 text-xs text-muted-foreground">
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
