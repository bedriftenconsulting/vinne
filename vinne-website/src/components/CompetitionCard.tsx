import { Link } from "react-router-dom";
import { Clock, Plus } from "lucide-react";
import { motion } from "framer-motion";
import { useCountdown } from "@/hooks/useCountdown";
import type { Competition } from "@/lib/competitions";

const CompetitionCard = ({ comp, index = 0 }: { comp: Competition; index?: number }) => {
  const { hours, minutes, seconds } = useCountdown(comp.endsAt);
  const pct = Math.round((comp.soldTickets / comp.totalTickets) * 100);

  const timeLabel = `${String(hours).padStart(2, "0")}:${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;

  return (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true }}
      transition={{ delay: index * 0.08, duration: 0.4 }}
    >
      <Link
        to={`/competitions/${comp.id}`}
        className="group block card-light rounded-xl overflow-hidden shadow-md hover:shadow-xl transition-shadow border border-black/8"
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
          {/* Badge — orange pill, matches reference */}
          <span className="absolute top-3 left-3 bg-[hsl(22_100%_52%)] text-white px-3 py-1 rounded-full text-[11px] font-bold uppercase tracking-wide flex items-center gap-1.5">
            {comp.tag === "LIVE" && (
              <span className="w-1.5 h-1.5 bg-white rounded-full animate-pulse inline-block" />
            )}
            {comp.tag === "LIVE"
              ? `CLOSES IN ${timeLabel}`
              : comp.tag === "Ending Soon"
              ? `ENDS ${timeLabel}`
              : comp.tag}
          </span>
        </div>

        <div className="p-4">
          <h3 className="font-heading text-base text-[hsl(0_0%_10%)] mb-3 line-clamp-2 leading-snug">
            {comp.title}
          </h3>

          <div className="flex items-center justify-between mb-3">
            <span className="text-[hsl(0_0%_10%)] font-bold text-xl">
              {comp.currency} {comp.ticketPrice.toFixed(2)}
            </span>
            <motion.button
              whileHover={{ scale: 1.12 }}
              whileTap={{ scale: 0.93 }}
              className="w-9 h-9 rounded-full bg-[hsl(22_100%_52%)] flex items-center justify-center shadow-md hover:brightness-110 transition"
              onClick={(e) => e.preventDefault()}
            >
              <Plus size={18} className="text-white" strokeWidth={2.5} />
            </motion.button>
          </div>

          {/* Progress bar */}
          <div className="mb-2">
            <div className="h-2 bg-black/10 rounded-full overflow-hidden">
              <div
                className="h-full rounded-full bg-gradient-to-r from-[hsl(0_80%_45%)] to-[hsl(22_100%_52%)] transition-all"
                style={{ width: `${pct}%` }}
              />
            </div>
          </div>

          <div className="flex items-center justify-between text-xs text-black/45">
            <span>{pct}% sold</span>
            <span className="flex items-center gap-1">
              <Clock size={11} />
              {timeLabel}
            </span>
          </div>
        </div>
      </Link>
    </motion.div>
  );
};

export default CompetitionCard;
