import Navbar from "@/components/Navbar";
import Footer from "@/components/Footer";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";

const faqs = [
  { q: "How does WinBig Africa work?", a: "Choose a competition, answer a simple question, and buy tickets. When the draw happens, a winner is randomly selected. It's that simple!" },
  { q: "How do I pay for tickets?", a: "We accept Mobile Money (MoMo) payments. Once you click 'Buy Tickets', a payment request will be sent to your phone. Confirm the payment and your tickets are instantly generated." },
  { q: "How are winners selected?", a: "Winners are selected using a certified random number generator. The draw happens automatically once all tickets are sold or the timer runs out." },
  { q: "How do I claim my prize?", a: "Winners are contacted via phone and email. Prizes are delivered within 14 business days. Cash prizes are sent directly to your mobile money account." },
  { q: "Is there a limit on tickets?", a: "Each competition may have different ticket limits per person. Check the competition details for specifics." },
  { q: "What if I don't win?", a: "Better luck next time! New competitions launch regularly. Keep playing for your chance to win big." },
];

const FAQPage = () => (
  <div className="min-h-screen bg-background">
    <Navbar />
    <div className="container pt-24 pb-16 max-w-3xl">
      <h1 className="font-heading text-4xl md:text-5xl text-primary mb-10">FAQ</h1>
      <Accordion type="single" collapsible className="space-y-3">
        {faqs.map((f, i) => (
          <AccordionItem key={i} value={`faq-${i}`} className="bg-card border border-border rounded-xl px-5">
            <AccordionTrigger className="text-foreground font-medium hover:text-primary py-5">{f.q}</AccordionTrigger>
            <AccordionContent className="text-muted-foreground pb-5">{f.a}</AccordionContent>
          </AccordionItem>
        ))}
      </Accordion>

      <div className="mt-16 bg-card border border-border rounded-xl p-8 text-center">
        <h2 className="font-heading text-2xl text-foreground mb-3">Still have questions?</h2>
        <p className="text-muted-foreground mb-2">📧 support@winbigafrica.com</p>
        <p className="text-muted-foreground">📞 +233 20 000 0000</p>
      </div>
    </div>
    <Footer />
  </div>
);

export default FAQPage;
