import { motion, useScroll, useTransform } from "framer-motion";
import { Link } from "react-router-dom";
import { useCountdown } from "@/hooks/useCountdown";
import { competitions } from "@/lib/competitions";
import { useRef } from "react";

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

const HeroSection = () => {
  const featured = competitions.find((c) => c.featured)!;
  const { hours, minutes, seconds } = useCountdown(featured.endsAt);
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
      {/* ── Full-section background video ── */}
      <motion.div style={{ y: bgY }} className="absolute inset-0 scale-110">
        <video
          autoPlay
          muted
          loop
          playsInline
          className="w-full h-full object-cover"
          style={{ mixBlendMode: "screen" }}
          poster={featured.image}
        >
          <source src="/large_2x.mp4" type="video/mp4" />
        </video>

        {/* Gradient: strong on left for text legibility, fades to almost transparent on right */}
        <div className="absolute inset-0 bg-gradient-to-r from-[hsl(0_0%_4%)] via-[hsl(0_0%_4%/0.72)] to-[hsl(0_0%_4%/0.15)]" />
        {/* Bottom fade into next section */}
        <div className="absolute inset-0 bg-gradient-to-t from-[hsl(0_0%_4%)] via-transparent to-transparent" />
        {/* Red tint to match brand */}
        <div className="absolute inset-0 bg-[hsl(0_80%_45%/0.08)]" />
      </motion.div>

      {/* Sparkles sit above video */}
      {SPARKLES.map((s, i) => <Sparkle key={i} {...s} />)}

      {/* ── Text content — left-aligned on desktop, centered stacked on mobile ── */}
      <motion.div
        style={{ y: textY, opacity: fadeOut }}
        className="container relative z-20 py-16"
      >
        {/* Mobile: flex-col centered. Desktop: left block max-w-lg */}
        <motion.div
          variants={containerVariants}
          initial="hidden"
          animate="visible"
          className="flex flex-col items-center text-center md:items-start md:text-left max-w-lg mx-auto md:mx-0"
        >

          {/* 1. Headline */}
          <motion.h1 variants={item} className="font-heading leading-none mb-6">
            <span className="block text-gold text-5xl md:text-6xl lg:text-7xl drop-shadow-[0_2px_16px_hsl(44_100%_50%/0.5)]">
              WIN AN
            </span>
            <span className="block text-gold text-5xl md:text-6xl lg:text-7xl drop-shadow-[0_2px_16px_hsl(44_100%_50%/0.5)]">
              {featured.title.toUpperCase()}
            </span>
          </motion.h1>

          {/* 2. Countdown */}
          <motion.div variants={item} className="mb-6 w-full">
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

          </motion.div>

          {/* 3. Buttons — always side by side */}
          <motion.div variants={item} className="flex flex-row gap-3 mb-5">
            <motion.div whileHover={{ scale: 1.04 }} whileTap={{ scale: 0.97 }}>
              <Link
                to={`/competitions/${featured.id}`}
                className="inline-flex items-center gap-2 bg-primary text-white font-heading text-lg md:text-xl px-8 md:px-12 py-4 rounded-lg btn-glow animate-pulse-glow hover:brightness-110 transition tracking-wide"
              >
                ENTER NOW
                <motion.span
                  animate={{ x: [0, 5, 0] }}
                  transition={{ duration: 1.2, repeat: Infinity, ease: "easeInOut" }}
                >
                  →
                </motion.span>
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

          {/* 4. Price note */}
          <motion.p variants={item} className="text-white/45 text-sm">
            Tickets from{" "}
            <span className="text-gold font-semibold">
              {featured.currency} {featured.ticketPrice.toFixed(2)}
            </span>
          </motion.p>

        </motion.div>
      </motion.div>
    </section>
  );
};

export default HeroSection;
