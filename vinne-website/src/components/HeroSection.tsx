import { motion, useScroll, useTransform } from "framer-motion";
import { Link } from "react-router-dom";
import { useCountdown } from "@/hooks/useCountdown";
import { useRef, useEffect, useState, useMemo } from "react";
import { fetchActiveGames, type ApiGame } from "@/lib/api";

const SPARKLES = [
  { top: "10%", left: "6%",  size: 18, delay: 0   },
  { top: "18%", left: "30%", size: 12, delay: 0.4 },
  { top: "7%",  left: "54%", size: 22, delay: 0.8 },
  { top: "14%", left: "74%", size: 14, delay: 0.2 },
  { top: "70%", left: "4%",  size: 16, delay: 1.0 },
  { top: "78%", left: "26%", size: 10, delay: 0.6 },
  { top: "44%", left: "87%", size: 20, delay: 0.3 },
  { top: "82%", left: "68%", size: 12, delay: 0.9 },
  { top: "32%", left: "91%", size: 16, delay: 0.5 },
];

const Sparkle = ({ top, left, size, delay }: { top: string; left: string; size: number; delay: number }) => (
  <motion.div
    className="absolute pointer-events-none select-none z-10"
    style={{ top, left }}
    animate={{ scale: [0.8, 1.4, 0.8], opacity: [0.4, 1, 0.4], rotate: [0, 20, 0] }}
    transition={{ duration: 2.4, repeat: Infinity, delay, ease: "easeInOut" }}
  >
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none">
      <path d="M12 2L13.5 9.5L21 11L13.5 12.5L12 20L10.5 12.5L3 11L10.5 9.5Z" fill="hsl(44 100% 52%)" />
    </svg>
  </motion.div>
);

const containerVariants = {
  hidden: {},
  visible: { transition: { staggerChildren: 0.12, delayChildren: 0.2 } },
};
const item = {
  hidden: { opacity: 0, y: 32 },
  visible: { opacity: 1, y: 0, transition: { duration: 0.65, ease: [0.22, 1, 0.36, 1] } },
};

// Get next draw date for a game
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

// Get prize label from prize_details JSON
const getPrizeLabel = (game: ApiGame): string => {
  try {
    const prizes = JSON.parse(game.prize_details || "[]");
    if (prizes[0]?.description) return prizes[0].description;
  } catch { /* ignore */ }
  return game.name;
};

// Inner component that uses the hook (must be called unconditionally)
const HeroContent = ({ game }: { game: ApiGame }) => {
  const drawDate = useMemo(() => getNextDrawDate(game), [game.id, game.draw_date, game.draw_time]);
  const { days, hours, minutes, seconds } = useCountdown(drawDate);
  const prizeLabel = getPrizeLabel(game);
  const ref = useRef<HTMLElement>(null);

  const { scrollYProgress } = useScroll({ target: ref, offset: ["start start", "end start"] });
  const bgY    = useTransform(scrollYProgress, [0, 1], ["0%", "25%"]);
  const textY  = useTransform(scrollYProgress, [0, 1], ["0%", "12%"]);
  const fadeOut = useTransform(scrollYProgress, [0, 0.8], [1, 0]);

  return (
    <section
      ref={ref}
      className="relative min-h-screen flex items-center overflow-hidden bg-[hsl(0_0%_4%)] pt-16"
    >
      <motion.div style={{ y: bgY }} className="absolute inset-0 scale-110">
        <video 
          autoPlay 
          muted 
          loop 
          playsInline 
          preload="auto"
          className="w-full h-full object-cover"
        >
          <source src="/large_2x.mp4" type="video/mp4" />
        </video>
        <div className="absolute inset-0 bg-gradient-to-r from-[hsl(0_0%_4%/0.85)] via-[hsl(0_0%_4%/0.45)] to-[hsl(0_0%_4%/0.05)]" />
        <div className="absolute inset-0 bg-gradient-to-t from-[hsl(0_0%_4%/0.7)] via-transparent to-transparent" />
        <div className="absolute inset-0 bg-[hsl(0_80%_45%/0.05)]" />
      </motion.div>

      {SPARKLES.map((s, i) => <Sparkle key={i} {...s} />)}

      <motion.div style={{ y: textY, opacity: fadeOut }} className="container relative z-20 py-16">
        <motion.div
          variants={containerVariants}
          initial="hidden"
          animate="visible"
          className="flex flex-col items-center text-center md:items-start md:text-left max-w-lg mx-auto md:mx-0"
        >
          <motion.h1 variants={item} className="font-heading leading-none mb-6">
            <span className="block text-gold text-4xl md:text-5xl lg:text-6xl drop-shadow-[0_2px_16px_hsl(44_100%_50%/0.5)]">
              WIN A
            </span>
            <span className="block text-gold text-4xl md:text-5xl lg:text-6xl drop-shadow-[0_2px_16px_hsl(44_100%_50%/0.5)]">
              {prizeLabel.toUpperCase()}
            </span>
          </motion.h1>

          <motion.div variants={item} className="mb-6 w-full">
            {days > 0 ? (
              /* Multi-day: show "X days left" badge + HH:MM:SS */
              <div className="space-y-2">
                <div className="flex flex-wrap items-center justify-center md:justify-start gap-2 mb-2">
                  <div className="inline-flex items-center gap-1.5 bg-emerald-500/90 backdrop-blur-sm text-white px-3 py-1.5 rounded-full text-xs font-semibold shadow-lg">
                    <div className="w-1.5 h-1.5 bg-white rounded-full animate-pulse" />
                    LIVE
                  </div>
                  <div className="inline-flex items-center gap-1.5 bg-slate-800/90 backdrop-blur-sm text-white px-3 py-1.5 rounded-full text-xs font-medium shadow-lg">
                    <svg className="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
                      <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm1-12a1 1 0 10-2 0v4a1 1 0 00.293.707l2.828 2.829a1 1 0 101.415-1.415L11 9.586V6z" clipRule="evenodd" />
                    </svg>
                    {days} {days === 1 ? "day" : "days"} left
                  </div>
                </div>
                <div className="flex items-center justify-center md:justify-start gap-1">
                  {[{ label: "HRS", value: hours }, { label: "MIN", value: minutes }, { label: "SEC", value: seconds }].map((t, i) => (
                    <span key={i} className="flex items-center">
                      <span className="flex flex-col items-center">
                        <motion.span
                          key={`${i}-${t.value}`}
                          initial={{ opacity: 0, y: -4 }}
                          animate={{ opacity: 1, y: 0 }}
                          className="font-heading text-gold text-4xl md:text-5xl tabular-nums drop-shadow-[0_0_20px_hsl(44_100%_50%/0.7)]"
                        >
                          {String(t.value).padStart(2, "0")}
                        </motion.span>
                        <span className="text-gold/50 text-[10px] font-heading tracking-widest">{t.label}</span>
                      </span>
                      {i < 2 && (
                        <motion.span
                          animate={{ opacity: [1, 0.2, 1] }}
                          transition={{ duration: 1, repeat: Infinity }}
                          className="font-heading text-gold text-4xl md:text-5xl mx-1 mb-4"
                        >
                          :
                        </motion.span>
                      )}
                    </span>
                  ))}
                </div>
              </div>
            ) : (
              /* Same-day: big HH:MM:SS countdown */
              <div className="space-y-1">
                <div className="flex flex-wrap items-center justify-center md:justify-start gap-2 mb-2">
                  <div className="inline-flex items-center gap-1.5 bg-emerald-500/90 backdrop-blur-sm text-white px-3 py-1.5 rounded-full text-xs font-semibold shadow-lg">
                    <div className="w-1.5 h-1.5 bg-white rounded-full animate-pulse" />
                    LIVE
                  </div>
                  <div className="inline-flex items-center gap-1.5 bg-amber-500/90 backdrop-blur-sm text-white px-3 py-1.5 rounded-full text-xs font-medium shadow-lg">
                    <svg className="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
                      <path fillRule="evenodd" d="M11.3 1.046A1 1 0 0112 2v5h4a1 1 0 01.82 1.573l-7 10A1 1 0 018 18v-5H4a1 1 0 01-.82-1.573l7-10a1 1 0 011.12-.38z" clipRule="evenodd" />
                    </svg>
                    Draw today
                  </div>
                </div>
                <div className="flex items-center justify-center md:justify-start gap-1">
                  {[{ value: hours }, { value: minutes }, { value: seconds }].map((t, i) => (
                    <span key={i} className="flex items-center">
                      <motion.span
                        key={`${i}-${t.value}`}
                        initial={{ opacity: 0, y: -6 }}
                        animate={{ opacity: 1, y: 0 }}
                        className="font-heading text-gold text-5xl md:text-6xl lg:text-7xl tabular-nums drop-shadow-[0_0_20px_hsl(44_100%_50%/0.7)]"
                      >
                        {String(t.value).padStart(2, "0")}
                      </motion.span>
                      {i < 2 && (
                        <motion.span
                          animate={{ opacity: [1, 0.2, 1] }}
                          transition={{ duration: 1, repeat: Infinity }}
                          className="font-heading text-gold text-5xl md:text-6xl lg:text-7xl mx-1"
                        >
                          :
                        </motion.span>
                      )}
                    </span>
                  ))}
                </div>
              </div>
            )}
          </motion.div>

          <motion.div variants={item} className="flex flex-row gap-3 mb-5">
            <motion.div whileHover={{ scale: 1.04 }} whileTap={{ scale: 0.97 }}>
              <Link
                to={`/competitions/${game.id}`}
                className="inline-flex items-center gap-2 bg-primary text-white font-heading text-lg md:text-xl px-8 md:px-12 py-4 rounded-lg btn-glow animate-pulse-glow hover:brightness-110 transition tracking-wide"
              >
                ENTER NOW
                <motion.span animate={{ x: [0, 5, 0] }} transition={{ duration: 1.2, repeat: Infinity, ease: "easeInOut" }}>→</motion.span>
              </Link>
            </motion.div>
            <motion.div whileHover={{ scale: 1.03 }} whileTap={{ scale: 0.97 }}>
              <Link
                to="/competitions"
                className="inline-flex items-center gap-2 border border-white/20 hover:border-gold/50 text-white/80 hover:text-gold font-heading text-lg md:text-xl px-6 md:px-8 py-4 rounded-lg bg-white/5 hover:bg-white/10 transition tracking-wide"
              >
                VIEW ALL
              </Link>
            </motion.div>
          </motion.div>

          <motion.p variants={item} className="text-white/45 text-sm">
            Tickets from <span className="text-gold font-semibold">GHS {game.base_price.toFixed(2)}</span>
          </motion.p>
        </motion.div>
      </motion.div>
    </section>
  );
};

// Empty hero when no games
const HeroEmpty = () => (
  <section className="relative min-h-[60vh] flex items-center justify-center bg-[hsl(0_0%_4%)] pt-16">
    <div className="text-center">
      <h1 className="font-heading text-4xl text-gold mb-4">WINBIG AFRICA</h1>
      <p className="text-white/50 mb-6">New competitions coming soon</p>
      <Link to="/competitions" className="bg-primary text-white font-heading px-8 py-3 rounded-lg btn-glow">
        VIEW COMPETITIONS
      </Link>
    </div>
  </section>
);

const HeroSection = () => {
  const [games, setGames] = useState<ApiGame[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchActiveGames().then(setGames).catch(console.error).finally(() => setLoading(false));
  }, []);

  if (loading) return <section className="min-h-screen bg-[hsl(0_0%_4%)]" />;
  if (games.length === 0) return <HeroEmpty />;

  // Feature the first active game
  const featured = games[0];
  return <HeroContent game={featured} />;
};

export default HeroSection;
