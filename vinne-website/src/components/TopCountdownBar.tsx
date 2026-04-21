import { useEffect, useState, useMemo } from "react";
import { fetchActiveGames, type ApiGame } from "@/lib/api";
import { useCountdown } from "@/hooks/useCountdown";

const getNextDrawDate = (game: ApiGame): Date => {
  if (game.draw_date) return new Date(game.draw_date + "T" + (game.draw_time || "20:00") + ":00Z");
  const [h, m] = (game.draw_time || "20:00").split(":").map(Number);
  const now = new Date();
  const next = new Date(now);
  next.setUTCHours(h, m, 0, 0);
  if (next <= now) next.setUTCDate(next.getUTCDate() + 1);
  return next;
};

// Single digit box
const DigitBox = ({ value, label }: { value: string; label: string }) => (
  <div className="flex items-center gap-0.5">
    {value.split("").map((d, i) => (
      <div key={i} className="flex flex-col items-center">
        <div className="w-7 h-8 bg-gray-100 rounded-md flex items-center justify-center">
          <span className="text-gray-700 font-semibold text-sm tabular-nums leading-none"
            style={{ fontFamily: "'Poppins', sans-serif" }}>
            {d}
          </span>
        </div>
      </div>
    ))}
    <span className="text-gray-400 text-[10px] ml-1 self-end mb-0.5"
      style={{ fontFamily: "'Poppins', sans-serif", writingMode: "vertical-rl", transform: "rotate(180deg)", lineHeight: 1 }}>
      {label}
    </span>
  </div>
);

const CountdownInner = ({ game }: { game: ApiGame }) => {
  const drawDate = useMemo(() => getNextDrawDate(game), [game.id, game.draw_date, game.draw_time]);
  const { days, hours, minutes, seconds } = useCountdown(drawDate);

  return (
    <div className="bg-white border-b border-gray-200 py-2 fixed top-0 left-0 right-0 z-50">
      <div className="container flex items-center justify-center gap-3">
        <span className="text-gray-500 text-sm font-medium mr-1"
          style={{ fontFamily: "'Poppins', sans-serif" }}>
          Ends in
        </span>
        <DigitBox value={String(days).padStart(2, "0")} label="Days" />
        <DigitBox value={String(hours).padStart(2, "0")} label="Hrs" />
        <DigitBox value={String(minutes).padStart(2, "0")} label="Mins" />
        <DigitBox value={String(seconds).padStart(2, "0")} label="Secs" />
      </div>
    </div>
  );
};

const TopCountdownBar = () => {
  const [game, setGame] = useState<ApiGame | null>(null);

  useEffect(() => {
    fetchActiveGames().then(games => { if (games[0]) setGame(games[0]); }).catch(() => {});
  }, []);

  if (!game) return null;
  return <CountdownInner game={game} />;
};

export default TopCountdownBar;
