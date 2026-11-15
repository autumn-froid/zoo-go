"use client";

import { ChevronDown } from "lucide-react";
import { useState } from "react";
import { Reveal } from "@/components/marketing/reveal";
import { FAQS } from "@/lib/faq";
import { cn } from "@/lib/utils";

export function Faq() {
  const [open, setOpen] = useState<number | null>(0);

  return (
    <section
      id="faq"
      className="scroll-mt-20 border-t border-border/60 bg-card py-24 sm:py-32"
    >
      <div className="mx-auto max-w-3xl px-4 sm:px-6">
        <Reveal>
          <h2 className="text-3xl font-bold tracking-tight text-foreground sm:text-4xl">
            先把关键顾虑说清楚
          </h2>
        </Reveal>
        <Reveal
          delay={0.08}
          className="mt-12 divide-y divide-border/70 border-y border-border/70"
        >
          {FAQS.map((item, index) => {
            const isOpen = open === index;
            const panelId = `faq-panel-${index}`;
            const buttonId = `faq-button-${index}`;

            return (
              <div key={item.q}>
                <button
                  id={buttonId}
                  type="button"
                  aria-expanded={isOpen}
                  aria-controls={panelId}
                  onClick={() => setOpen(isOpen ? null : index)}
                  className={cn(
                    "flex w-full items-center justify-between gap-4 py-5 text-left text-base outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring/60 sm:text-lg",
                    isOpen
                      ? "font-semibold text-foreground"
                      : "font-medium text-foreground/90 hover:text-foreground",
                  )}
                >
                  {item.q}
                  <ChevronDown
                    className={cn(
                      "size-5 shrink-0 text-muted-foreground transition-transform duration-200",
                      isOpen && "rotate-180",
                    )}
                    aria-hidden
                  />
                </button>
                {/* Keep answers in the HTML for crawlers; collapse with CSS only. */}
                <div
                  id={panelId}
                  role="region"
                  aria-labelledby={buttonId}
                  className={cn(
                    "grid transition-[grid-template-rows] duration-200 ease-out",
                    isOpen ? "grid-rows-[1fr]" : "grid-rows-[0fr]",
                  )}
                >
                  <div className="overflow-hidden">
                    <p className="max-w-prose pb-5 text-sm leading-relaxed text-muted-foreground">
                      {item.a}
                    </p>
                  </div>
                </div>
              </div>
            );
          })}
        </Reveal>
      </div>
    </section>
  );
}
