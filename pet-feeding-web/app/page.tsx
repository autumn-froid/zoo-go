import { Coverage } from "@/components/marketing/coverage";
import { Faq } from "@/components/marketing/faq";
import { Hero } from "@/components/marketing/hero";
import { HowItWorks } from "@/components/marketing/how-it-works";
import { JsonLd } from "@/components/marketing/json-ld";
import { LeadSection } from "@/components/marketing/lead-form";
import { SiteFooter } from "@/components/marketing/site-footer";
import { SiteHeader } from "@/components/marketing/site-header";
import { StudentCta } from "@/components/marketing/student-cta";
import { Trust } from "@/components/marketing/trust";

export default function Home() {
  return (
    <>
      <JsonLd />
      <SiteHeader />
      <main>
        <Hero />
        <HowItWorks />
        <Trust />
        <Coverage />
        <StudentCta />
        <LeadSection />
        <Faq />
      </main>
      <SiteFooter />
    </>
  );
}
