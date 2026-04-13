import Navbar from "@/components/Navbar";
import Footer from "@/components/Footer";
import { Trophy } from "lucide-react";
import { winners } from "@/lib/competitions";
import { motion } from "framer-motion";

const ResultsPage = () => (
  <div className="min-h-screen bg-background">
    <Navbar />
    <div className="container pt-24 pb-16">
      <h1 className="font-heading text-4xl md:text-5xl text-primary mb-10">WINNERS & RESULTS</h1>

      <div className="grid gap-4 max-w-2xl">
        {winners.map((w, i) => (
          <motion.div
            key={w.ticket}
            initial={{ opacity: 0, x: -20 }}
            animate={{ opacity: 1, x: 0 }}
            transition={{ delay: i * 0.1 }}
            className="bg-card border border-border rounded-xl p-5 flex items-center gap-4"
          >
            <div className="w-12 h-12 rounded-full bg-primary/10 flex items-center justify-center shrink-0">
              <Trophy className="text-primary" size={24} />
            </div>
            <div className="flex-1 min-w-0">
              <div className="font-heading text-foreground text-lg">{w.name}</div>
              <div className="text-muted-foreground text-sm">{w.prize}</div>
            </div>
            <div className="text-right shrink-0">
              <div className="text-primary font-mono text-sm">{w.ticket}</div>
              <div className="text-muted-foreground text-xs">{w.date}</div>
            </div>
          </motion.div>
        ))}
      </div>
    </div>
    <Footer />
  </div>
);

export default ResultsPage;
