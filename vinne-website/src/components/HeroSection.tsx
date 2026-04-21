import { motion, useScroll, useTransform } from "framer-motion";
import { Link } from "react-router-dom";
import { useCountdown } from "@/hooks/useCountdown";
import { useRef, useEffect, useState, useMemo } from "react";
import { fetchActiveGames, type ApiGame } from "@/lib/api";
import { Trophy, X } from "lucide-react";

// Get next draw date
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

const getPrizeLabel = (game: ApiGame): string => {
  try {
    const prizes = JSON.parse(game.prize_details || "[]");
    if (prizes[0]?.description) return prizes[0].description;
  } catch { /* ignore */ }
  return game.name;
};

const item = {
  hidden: { opacity: 0, y: 24 },
  visible: { opacity: 1, y: 0, transition: { duration: 0.55, ease: [0.22, 1, 0.36, 1] } },
};
const containerVariants = {
  hidden: {},
  visible: { transition: { staggerChildren: 0.1, delayChildren: 0.15 } },
};

const HeroContent = ({ game }: { game: ApiGame }) => {
  const drawDate = useMemo(() => getNextDrawDate(game), [game.id, game.draw_date, game.draw_time]);
  const { days, hours, minutes, seconds } = useCountdown(drawDate);
  const prizeLabel = getPrizeLabel(game);
  const ref = useRef<HTMLElement>(null);
  const [bannerDismissed, setBannerDismissed] = useState(false);

  const { scrollYProgress } = useScroll({ target: ref, offset: ["start start", "end start"] });
  const bgY = useTransform(scrollYProgress, [0, 1], ["0%", "20%"]);
  const fadeOut = useTransform(scrollYProgress, [0, 0.7], [1, 0]);

  const timeLabel = days > 0
    ? `${days}D ${String(hours).padStart(2,"0")}H ${String(minutes).padStart(2,"0")}M`
    : `${String(hours).padStart(2,"0")}:${String(minutes).padStart(2,"0")}:${String(seconds).padStart(2,"0")}`;

  // Banner always uses the same featured game as the hero
  const bannerPrize = getPrizeLabel(game);

  return (
    <>
      <section ref={ref} className="relative min-h-screen flex flex-col overflow-hidden bg-[hsl(0_0%_4%)] pt-28">
        {/* Video background */}
        <motion.div style={{ y: bgY }} className="absolute inset-0 scale-110">
          <video autoPlay muted loop playsInline className="w-full h-full object-cover opacity-70">
            <source src="/large_2x.mp4" type="video/mp4" />
          </video>
          <div className="absolute inset-0 bg-gradient-to-r from-[hsl(0_0%_4%/0.85)] via-[hsl(0_0%_4%/0.4)] to-transparent" />
          <div className="absolute inset-0 bg-gradient-to-t from-[hsl(0_0%_4%/0.7)] via-transparent to-transparent" />
        </motion.div>

        {/* Price tag — absolute top right */}
        <motion.div
          initial={{ opacity: 0, scale: 0.8 }}
          animate={{ opacity: 1, scale: 1 }}
          transition={{ duration: 0.6, ease: [0.22, 1, 0.36, 1], delay: 0.4 }}
          className="absolute top-24 right-8 z-30 hidden md:block"
        >
          <motion.div
            animate={{ rotate: [-2, 2, -2], y: [-4, 4, -4] }}
            transition={{ duration: 5, repeat: Infinity, ease: "easeInOut" }}
            className="relative text-center"
            style={{ filter: "drop-shadow(0 0 30px hsl(0 80% 45% / 0.7))" }}
          >
            <div className="absolute -top-3 left-1/2 -translate-x-1/2 w-6 h-6 rounded-full z-10"
              style={{ background: "#111", border: "3px solid #cc0000" }} />
            <div className="bg-primary text-white font-heading px-8 py-6 rounded-2xl"
              style={{ boxShadow: "0 0 40px hsl(0 80% 45% / 0.5)" }}>
              <div className="text-xs tracking-widest opacity-90 mt-2">TICKETS JUST</div>
              <div className="text-6xl leading-none font-black my-1">{game.base_price.toFixed(0)}</div>
              <div className="text-base tracking-widest opacity-90">GHS</div>
            </div>
          </motion.div>
        </motion.div>

        {/* Main content */}
        <motion.div style={{ opacity: fadeOut }} className="relative z-20 flex-1 flex items-center">
          <div className="container py-12 md:py-16">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-8 items-center">

              {/* LEFT — text + CTA */}
              <motion.div
                variants={containerVariants}
                initial="hidden"
                animate="visible"
                className="flex flex-col"
              >
                {/* Category tag — hidden */}

                {/* Headline */}
                <motion.h1 variants={item} className="font-heading leading-[0.95] mb-4">
                  <span className="block text-white text-3xl md:text-4xl lg:text-5xl font-bold">WIN A</span>
                  <span className="block text-gold text-4xl md:text-5xl lg:text-6xl font-black italic drop-shadow-[0_2px_24px_hsl(44_100%_50%/0.6)]">
                    {prizeLabel.toUpperCase()}
                  </span>
                </motion.h1>

                {/* Subtext */}
                {game.description && (
                  <motion.p variants={item} className="text-white/50 text-sm mb-5 max-w-sm leading-relaxed">
                    {game.description}
                  </motion.p>
                )}

                {/* Countdown */}
                <motion.div variants={item} className="mb-5">
                  {/* ENDS badge — gradient pill like BOTB */}
                  <div className="mb-3">
                    {days === 0 ? (
                      <span className="inline-block font-bold text-white px-3 py-1 rounded-lg shadow-lg"
                        style={{ background: "linear-gradient(90deg, #ff0080, #ff6000)", fontFamily: "'Poppins', sans-serif", fontSize: "0.72rem" }}>
                        ENDS TODAY
                      </span>
                    ) : days === 1 ? (
                      <span className="inline-block font-bold text-white px-3 py-1 rounded-lg shadow-lg"
                        style={{ background: "linear-gradient(90deg, #ff0080, #ff6000)", fontFamily: "'Poppins', sans-serif", fontSize: "0.72rem" }}>
                        ENDS TOMORROW
                      </span>
                    ) : days <= 3 ? (
                      <span className="inline-block font-bold text-white px-3 py-1 rounded-lg shadow-lg"
                        style={{ background: "linear-gradient(90deg, #ff0080, #ff6000)", fontFamily: "'Poppins', sans-serif", fontSize: "0.72rem" }}>
                        ENDS IN {days} DAYS
                      </span>
                    ) : (
                      <span className="inline-block font-bold text-white px-3 py-1 rounded-lg shadow-lg"
                        style={{ background: "linear-gradient(90deg, #ff0080, #ff6000)", fontFamily: "'Poppins', sans-serif", fontSize: "0.72rem" }}>
                        ENDS {new Date(drawDate).toLocaleDateString("en-GB", { weekday: "short", day: "numeric", month: "short" }).toUpperCase()}
                      </span>
                    )}
                  </div>
                  <div className="flex items-end gap-1">
                    {[
                      { label: "HRS", value: hours },
                      { label: "MIN", value: minutes },
                      { label: "SEC", value: seconds },
                    ].map((t, i) => (
                      <span key={i} className="flex items-end">
                        <span className="flex flex-col items-center">
                          <motion.span
                            key={t.value}
                            initial={{ opacity: 0, y: -4 }}
                            animate={{ opacity: 1, y: 0 }}
                            className="font-heading text-gold text-5xl md:text-6xl tabular-nums leading-none drop-shadow-[0_0_20px_hsl(44_100%_50%/0.7)]"
                          >
                            {String(t.value).padStart(2, "0")}
                          </motion.span>
                          <span className="text-gold/40 text-[9px] font-heading tracking-widest mt-0.5">{t.label}</span>
                        </span>
                        {i < 2 && (
                          <motion.span
                            animate={{ opacity: [1, 0.2, 1] }}
                            transition={{ duration: 1, repeat: Infinity }}
                            className="font-heading text-gold text-5xl md:text-6xl mx-1 leading-none mb-4"
                          >:
                          </motion.span>
                        )}
                      </span>
                    ))}
                  </div>
                </motion.div>

                {/* CTAs */}
                <motion.div variants={item} className="flex items-center gap-3 mb-4">
                  <Link
                    to={`/competitions/${game.id}`}
                    className="inline-flex items-center gap-3 font-heading text-base md:text-lg px-8 py-3.5 rounded-lg tracking-widest transition-all duration-300 text-white hover:text-gold"
                    style={{
                      border: "1.5px solid hsl(44 100% 52% / 0.7)",
                      boxShadow: "0 0 18px hsl(44 100% 52% / 0.25), inset 0 0 18px hsl(44 100% 52% / 0.05)",
                      background: "transparent",
                    }}
                    onMouseEnter={e => (e.currentTarget.style.boxShadow = "0 0 32px hsl(44 100% 52% / 0.6), inset 0 0 24px hsl(44 100% 52% / 0.12)")}
                    onMouseLeave={e => (e.currentTarget.style.boxShadow = "0 0 18px hsl(44 100% 52% / 0.25), inset 0 0 18px hsl(44 100% 52% / 0.05)")}
                  >
                    ENTER TO WIN
                    <motion.span animate={{ x: [0, 4, 0] }} transition={{ duration: 1.2, repeat: Infinity }}>»</motion.span>
                  </Link>
                  <Link
                    to="/competitions"
                    className="inline-flex items-center gap-2 border border-white/20 hover:border-white/40 text-white/60 hover:text-white font-heading text-base md:text-lg px-6 py-3.5 rounded-lg bg-white/5 hover:bg-white/10 transition tracking-wide"
                  >
                    VIEW ALL
                  </Link>
                </motion.div>

                {/* Price */}
                <motion.p variants={item} className="text-white/40 text-sm">
                  Tickets from <span className="text-gold font-bold text-base">GHS {game.base_price.toFixed(2)}</span>
                </motion.p>
              </motion.div>

            </div>
          </div>
        </motion.div>

      </section>

      {/* Sticky bottom banner — BOTB exact style */}
      {!bannerDismissed && (
        <motion.div
          initial={{ y: 80 }}
          animate={{ y: 0 }}
          transition={{ delay: 1.5, duration: 0.5, ease: [0.22, 1, 0.36, 1] }}
          className="fixed bottom-0 left-0 right-0 z-50"
          style={{ background: "linear-gradient(90deg, #c43a00 0%, #e86000 40%, #f07800 100%)" }}
        >
          <div className="flex items-center justify-between px-4 py-3 gap-3">
            {/* Left — emoji + text */}
            <div className="flex items-center gap-2 min-w-0 flex-1">
              <span className="text-lg shrink-0">🏆</span>
              <p className="text-white font-semibold text-sm md:text-base truncate">
                Win a {bannerPrize}!
              </p>
            </div>

            {/* Center — price pill */}
            <div className="shrink-0">
              <span className="bg-[#1a1a1a] text-white font-bold text-sm px-4 py-1.5 rounded-full">
                Only GHS {game.base_price.toFixed(0)}
              </span>
            </div>

            {/* Right — CTA + close */}
            <div className="flex items-center gap-2 shrink-0">
              <Link
                to={`/competitions/${game.id}`}
                className="bg-white text-[#c43a00] font-heading font-bold text-sm px-6 py-2 rounded-lg hover:bg-orange-50 transition tracking-wide"
              >
                ENTER NOW
              </Link>
              <button onClick={() => setBannerDismissed(true)}
                className="text-white/70 hover:text-white transition p-1 ml-1">
                <X size={16} />
              </button>
            </div>
          </div>
        </motion.div>
      )}
    </>
  );
};

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

  return <HeroContent game={games[0]} />;
};

export default HeroSection;
