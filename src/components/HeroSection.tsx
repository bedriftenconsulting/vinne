import { motion } from "framer-motion";
import { Link } from "react-router-dom";
import { useCountdown } from "@/hooks/useCountdown";
import { competitions } from "@/lib/competitions";

const HeroSection = () => {
  const featured = competitions.find((c) => c.featured)!;
  const { hours, minutes, seconds } = useCountdown(featured.endsAt);

  return (
    <section className="relative min-h-[90vh] flex items-center overflow-hidden pt-16">
      <div className="absolute inset-0">
        <img src={featured.image} alt={featured.title} className="w-full h-full object-cover opacity-60" width={1920} height={1080} />
        <div className="absolute inset-0 bg-gradient-to-r from-background via-background/80 to-transparent" />
        <div className="absolute inset-0 bg-gradient-to-t from-background via-transparent to-transparent" />
      </div>

      <div className="container relative z-10">
        <motion.div
          initial={{ opacity: 0, y: 30 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.7 }}
          className="max-w-2xl"
        >
          <span className="inline-block bg-primary/20 text-primary px-4 py-1 rounded-full text-sm font-semibold mb-4 border border-primary/30">
            🏆 Africa's #1 Competition Platform
          </span>
          <h1 className="font-heading text-5xl md:text-7xl lg:text-8xl text-foreground leading-none mb-6">
            WIN AN<br />
            <span className="text-gradient-gold">iPHONE 17<br />PRO MAX!</span>
          </h1>

          <div className="flex gap-3 mb-8">
            {[
              { label: "HRS", value: hours },
              { label: "MIN", value: minutes },
              { label: "SEC", value: seconds },
            ].map((t) => (
              <div key={t.label} className="bg-card/80 backdrop-blur border border-border rounded-lg px-4 py-3 text-center min-w-[70px]">
                <div className="font-heading text-2xl md:text-3xl text-primary">{String(t.value).padStart(2, "0")}</div>
                <div className="text-[10px] text-muted-foreground tracking-widest">{t.label}</div>
              </div>
            ))}
          </div>

          <Link
            to={`/competitions/${featured.id}`}
            className="inline-block bg-primary text-primary-foreground font-heading text-lg md:text-xl px-10 py-4 rounded-lg btn-glow animate-pulse-glow hover:brightness-110 transition"
          >
            ENTER NOW
          </Link>

          <p className="mt-4 text-muted-foreground text-sm">
            Tickets from <span className="text-primary font-semibold">{featured.currency} {featured.ticketPrice.toFixed(2)}</span> • {Math.round((featured.soldTickets / featured.totalTickets) * 100)}% sold
          </p>
        </motion.div>
      </div>
    </section>
  );
};

export default HeroSection;
