import { Link } from "react-router-dom";
import { Clock, Plus } from "lucide-react";
import { motion } from "framer-motion";
import { useCountdown } from "@/hooks/useCountdown";
import type { Competition } from "@/lib/competitions";

const CompetitionCard = ({ comp, index = 0 }: { comp: Competition; index?: number }) => {
  const { hours, minutes, seconds } = useCountdown(comp.endsAt);
  const pct = Math.round((comp.soldTickets / comp.totalTickets) * 100);

  return (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true }}
      transition={{ delay: index * 0.1, duration: 0.4 }}
    >
      <Link
        to={`/competitions/${comp.id}`}
        className="group block bg-card rounded-xl overflow-hidden border border-border hover:border-primary/40 transition-all hover:glow-card"
      >
        <div className="relative aspect-[4/3] overflow-hidden">
          <img
            src={comp.image}
            alt={comp.title}
            className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-500"
            loading="lazy"
            width={800}
            height={600}
          />
          <span
            className={`absolute top-3 left-3 px-3 py-1 rounded-full text-xs font-bold uppercase tracking-wide ${
              comp.tag === "LIVE"
                ? "bg-green-500/90 text-foreground"
                : comp.tag === "Ending Soon"
                ? "bg-accent text-accent-foreground"
                : comp.tag === "Coming Soon"
                ? "bg-blue-500/90 text-foreground"
                : "bg-secondary text-secondary-foreground"
            }`}
          >
            {comp.tag === "LIVE" && <span className="inline-block w-2 h-2 bg-foreground rounded-full mr-1.5 animate-pulse" />}
            {comp.tag}
          </span>
        </div>

        <div className="p-4">
          <h3 className="font-heading text-lg text-foreground mb-2 line-clamp-1">{comp.title}</h3>

          <div className="flex items-center justify-between mb-3">
            <span className="text-primary font-bold text-lg">
              {comp.currency}{comp.ticketPrice.toFixed(2)}
            </span>
            <div className="flex items-center gap-1 text-muted-foreground text-xs">
              <Clock size={12} />
              {String(hours).padStart(2, "0")}:{String(minutes).padStart(2, "0")}:{String(seconds).padStart(2, "0")}
            </div>
          </div>

          <div className="mb-2">
            <div className="h-2 bg-secondary rounded-full overflow-hidden">
              <div
                className="h-full rounded-full bg-gradient-to-r from-primary to-accent transition-all"
                style={{ width: `${pct}%` }}
              />
            </div>
          </div>

          <div className="flex items-center justify-between">
            <span className="text-muted-foreground text-xs">{pct}% sold</span>
            <div className="w-8 h-8 rounded-full bg-primary flex items-center justify-center group-hover:scale-110 transition-transform">
              <Plus size={16} className="text-primary-foreground" />
            </div>
          </div>
        </div>
      </Link>
    </motion.div>
  );
};

export default CompetitionCard;
