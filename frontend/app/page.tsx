import Navbar from "@/components/Navbar";
import Hero from "@/components/Hero";
import FeatureCards from "@/components/FeatureCards";
import Footer from "@/components/Footer";

export default function HomePage() {
  return (
    <main className="flex min-h-screen flex-col">
      <Navbar />
      <div className="flex-1">
        <Hero />
        <FeatureCards />
      </div>
      <Footer />
    </main>
  );
}
