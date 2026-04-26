import { useEffect, useState, useMemo } from "react";
import { fetchActiveGames, type ApiGame } from "@/lib/api";
import { useCountdown } from "@/hooks/useCountdown";
import { API_BASE } from "@/lib/config";

const getNextDrawDate = (game: ApiGame): Date => {
  if (game.draw_date) return new Date(game.draw_date + "T" + (game.draw_time || "20:00") + ":00Z");
  const [h, m] = (game.draw_time || "20:00").split(":").map(Number);
  const now = new Date();
  const next = new Date(now);
  next.setUTCHours(h, m, 0, 0);
  if (next <= now) next.setUTCDate(next.getUTCDate() + 1);
  return next;
};

// Single white digit box
const Digit = ({ value }: { value: string }) => (
  <div className="w-8 h-9 bg-white rounded-lg flex items-center justify-center shadow-sm border border-gray-200">
    <span className="text-gray-700 font-semibold text-base tabular-nums leading-none select-none"
      style={{ fontFamily: "'Poppins', sans-serif" }}>
      {value}
    </span>
  </div>
);

// Rotated label between digit pairs
const Label = ({ text }: { text: string }) => (
  <span className="text-gray-400 text-[10px] font-medium mx-0.5 self-center"
    style={{
      fontFamily: "'Poppins', sans-serif",
      writingMode: "vertical-rl",
      transform: "rotate(180deg)",
      lineHeight: 1,
      letterSpacing: "0.05em",
    }}>
    {text}
  </span>
);

const CountdownInner = ({ game }: { game: ApiGame }) => {
  const drawDate = useMemo(() => getNextDrawDate(game), [game.id, game.draw_date, game.draw_time]);
  const { days, hours, minutes, seconds } = useCountdown(drawDate);

  const dd = String(days).padStart(2, "0");
  const hh = String(hours).padStart(2, "0");
  const mm = String(minutes).padStart(2, "0");
  const ss = String(seconds).padStart(2, "0");

  return (
    <div className="fixed top-0 left-0 right-0 z-50 border-b border-gray-200"
      style={{ background: "#f0f0f0" }}>
      <div className="flex items-center justify-center gap-1.5 py-2 px-4">
        <span className="text-gray-500 text-sm font-medium mr-2"
          style={{ fontFamily: "'Poppins', sans-serif" }}>
          Ends in
        </span>

        {/* Days */}
        <Digit value={dd[0]} />
        <Digit value={dd[1]} />
        <Label text="Days" />

        {/* Hours */}
        <Digit value={hh[0]} />
        <Digit value={hh[1]} />
        <Label text="Hrs" />

        {/* Minutes */}
        <Digit value={mm[0]} />
        <Digit value={mm[1]} />
        <Label text="Mins" />

        {/* Seconds */}
        <Digit value={ss[0]} />
        <Digit value={ss[1]} />
        <Label text="Secs" />
      </div>
    </div>
  );
};

const TopCountdownBar = () => {
  const [game, setGame] = useState<ApiGame | null>(null);

  useEffect(() => {
    const configUrl = `${API_BASE}/config`;
    Promise.all([
      fetchActiveGames(),
      fetch(configUrl).then(r => r.json()).catch(() => ({})),
    ])
      .then(([games, cfg]) => {
        const featuredId = cfg?.featured_game_id;
        const hero = (featuredId && games.find((g: ApiGame) => g.id === featuredId)) || games[0];
        if (hero) setGame(hero);
      })
      .catch(() => {});
  }, []);

  if (!game) return null;
  return <CountdownInner game={game} />;
};

export default TopCountdownBar;
