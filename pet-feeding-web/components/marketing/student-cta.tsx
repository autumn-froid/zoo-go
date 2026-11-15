"use client";

import Image from "next/image";
import { motion } from "motion/react";
import { Reveal } from "@/components/marketing/reveal";
import { PET_IMAGES } from "@/lib/images";
import { onHashLinkClick } from "@/lib/scroll-to-id";

export function StudentCta() {
  return (
    <section id="students" className="scroll-mt-20 px-4 py-8 sm:px-6 sm:py-10">
      <Reveal>
        <div className="relative mx-auto max-w-6xl overflow-hidden rounded-3xl bg-trust text-trust-foreground shadow-md">
          <div className="pointer-events-none absolute inset-y-0 right-0 w-full sm:w-[66%] lg:w-[54%]">
            <Image
              src={PET_IMAGES.sofaFriends}
              alt="沙发上相依的狗狗与猫咪"
              fill
              className="object-cover object-[58%_42%]"
              sizes="(max-width: 640px) 100vw, (max-width: 1024px) 66vw, 620px"
            />
            <div
              className="absolute inset-0 hidden sm:block"
              style={{
                background:
                  "linear-gradient(90deg, var(--trust) 0%, color-mix(in srgb, var(--trust) 92%, transparent) 12%, color-mix(in srgb, var(--trust) 50%, transparent) 28%, color-mix(in srgb, var(--trust) 14%, transparent) 44%, transparent 60%)",
              }}
            />
            <div
              className="absolute inset-0 sm:hidden"
              style={{
                background:
                  "linear-gradient(to top, var(--trust) 0%, var(--trust) 46%, color-mix(in srgb, var(--trust) 72%, transparent) 70%, color-mix(in srgb, var(--trust) 18%, transparent) 100%)",
              }}
            />
          </div>

          <div className="relative min-h-[18rem] px-6 py-12 sm:min-h-0 sm:max-w-[52%] sm:px-10 sm:py-16 lg:py-20">
            <h2 className="text-3xl font-bold tracking-tight sm:text-4xl">
              用碎片时间，服务附近毛孩子
            </h2>
            <p className="mt-4 max-w-md text-base leading-relaxed text-white/85">
              普通人完成成年实名核验后即可成为宠护师，浏览附近任务并报价。报价时可见预计到手金额；履约后按规则结算。
            </p>
            <motion.div
              className="mt-8 inline-flex"
              whileHover={{ y: -1 }}
              whileTap={{ scale: 0.97 }}
            >
              <a
                href="#lead"
                onClick={(e) =>
                  onHashLinkClick(e, "lead", { leadRole: "student" })
                }
                className="inline-flex h-11 items-center justify-center rounded-full bg-white px-5 text-sm font-semibold text-trust shadow-sm transition hover:bg-white/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/70"
              >
                申请成为宠护师
              </a>
            </motion.div>
          </div>
        </div>
      </Reveal>
    </section>
  );
}
