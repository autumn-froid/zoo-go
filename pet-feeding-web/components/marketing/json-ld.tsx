import { FAQS } from "@/lib/faq";
import {
  SITE_DESCRIPTION,
  SITE_NAME,
  SITE_OG_DESCRIPTION,
  getSiteUrl,
} from "@/lib/site";

export function JsonLd() {
  const siteUrl = getSiteUrl();

  const graph = [
    {
      "@type": "WebSite",
      "@id": `${siteUrl}/#website`,
      name: SITE_NAME,
      url: siteUrl,
      description: SITE_DESCRIPTION,
      inLanguage: "zh-CN",
    },
    {
      "@type": "ProfessionalService",
      "@id": `${siteUrl}/#service`,
      name: SITE_NAME,
      url: siteUrl,
      description: SITE_OG_DESCRIPTION,
      areaServed: {
        "@type": "Country",
        name: "中国各省市",
      },
      serviceType: "上门宠物喂养",
      availableLanguage: "zh-CN",
    },
    {
      "@type": "FAQPage",
      "@id": `${siteUrl}/#faq`,
      mainEntity: FAQS.map((item) => ({
        "@type": "Question",
        name: item.q,
        acceptedAnswer: {
          "@type": "Answer",
          text: item.a,
        },
      })),
    },
  ];

  const payload = {
    "@context": "https://schema.org",
    "@graph": graph,
  };

  return (
    <script
      type="application/ld+json"
      dangerouslySetInnerHTML={{ __html: JSON.stringify(payload) }}
    />
  );
}
