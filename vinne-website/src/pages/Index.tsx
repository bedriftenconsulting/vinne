import Navbar from "@/components/Navbar";
import HeroSection from "@/components/HeroSection";
import LiveCompetitions from "@/components/LiveCompetitions";
import HowItWorks from "@/components/HowItWorks";
import Footer from "@/components/Footer";

const Index = () => (
  <div className="min-h-screen bg-background">
    <Navbar />
    <HeroSection />
    <LiveCompetitions />
    <HowItWorks />
    <Footer />
  </div>
);

export default Index;
