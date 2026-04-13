import Navbar from "@/components/Navbar";
import Footer from "@/components/Footer";
import CompetitionCard from "@/components/CompetitionCard";
import { competitions } from "@/lib/competitions";

const CompetitionsPage = () => {
  const active = competitions.filter((c) => c.tag === "LIVE");
  const ending = competitions.filter((c) => c.tag === "Ending Soon");
  const upcoming = competitions.filter((c) => c.tag === "Coming Soon");

  return (
    <div className="min-h-screen bg-background">
      <Navbar />
      <div className="container pt-24 pb-16">
        <h1 className="font-heading text-4xl md:text-5xl text-primary mb-10">ALL COMPETITIONS</h1>

        {ending.length > 0 && (
          <>
            <h2 className="font-heading text-2xl text-accent mb-4">🔥 Ending Soon</h2>
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6 mb-12">
              {ending.map((c, i) => <CompetitionCard key={c.id} comp={c} index={i} />)}
            </div>
          </>
        )}

        {active.length > 0 && (
          <>
            <h2 className="font-heading text-2xl text-primary mb-4">🎯 Active</h2>
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6 mb-12">
              {active.map((c, i) => <CompetitionCard key={c.id} comp={c} index={i} />)}
            </div>
          </>
        )}

        {upcoming.length > 0 && (
          <>
            <h2 className="font-heading text-2xl text-muted-foreground mb-4">⏳ Upcoming</h2>
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6">
              {upcoming.map((c, i) => <CompetitionCard key={c.id} comp={c} index={i} />)}
            </div>
          </>
        )}
      </div>
      <Footer />
    </div>
  );
};

export default CompetitionsPage;
