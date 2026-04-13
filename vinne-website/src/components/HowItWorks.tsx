import { motion } from "framer-motion";
import { Ticket, HelpCircle, Trophy } from "lucide-react";

const steps = [
  { icon: Ticket, title: "CHOOSE A PRIZE", desc: "Browse our exciting competitions and pick the prize you want to win." },
  { icon: HelpCircle, title: "ANSWER & BUY TICKETS", desc: "Answer a simple question and purchase your tickets to enter the draw." },
  { icon: Trophy, title: "WINNER IS DRAWN", desc: "A lucky winner is randomly selected and announced. Could it be you?" },
];

const HowItWorks = () => {
  return (
    <section className="py-16 bg-card/50">
      <div className="container">
        <h2 className="font-heading text-3xl md:text-4xl text-primary text-center mb-12">HOW IT WORKS</h2>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-8 max-w-4xl mx-auto">
          {steps.map((step, i) => (
            <motion.div
              key={step.title}
              initial={{ opacity: 0, y: 20 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
              transition={{ delay: i * 0.15 }}
              className="text-center"
            >
              <div className="w-20 h-20 mx-auto mb-4 rounded-2xl bg-primary/10 border border-primary/20 flex items-center justify-center">
                <step.icon className="text-primary" size={36} />
              </div>
              <div className="text-muted-foreground text-sm font-bold mb-1">{i + 1}</div>
              <h3 className="font-heading text-lg text-foreground mb-2">{step.title}</h3>
              <p className="text-muted-foreground text-sm leading-relaxed">{step.desc}</p>
            </motion.div>
          ))}
        </div>
      </div>
    </section>
  );
};

export default HowItWorks;
