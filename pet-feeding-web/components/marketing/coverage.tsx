"use client";

import Image from "next/image";
import { motion, useReducedMotion } from "motion/react";
import { Reveal, easeOut, viewport } from "@/components/marketing/reveal";
import { PET_IMAGES } from "@/lib/images";
import { onHashLinkClick } from "@/lib/scroll-to-id";

const areas = [
  "马连洼·朗润园附近",
  "马连洼·农大南路附近",
  "马连洼·圆明园西路附近",
];

export function Coverage() {
  const reduceMotion = useReducedMotion();

  return (
    <section
      id="coverage"
      className="scroll-mt-20 border-t border-border/60 py-24 sm:py-32"
    >
      <div className="mx-auto grid max-w-6xl gap-12 px-4 sm:px-6 lg:grid-cols-[1.05fr_0.95fr] lg:items-center lg:gap-16">
        <Reveal>
          <h2 className="text-3xl font-bold tracking-tight text-foreground sm:text-4xl">
            覆盖各省各市已开通小区
          </h2>
          <p className="mt-4 max-w-prose text-muted-foreground leading-relaxed">
            当前服务覆盖各省各市已开通小区。未在开放范围内的地址无法发单——若你的小区尚未开通，留下联系方式，我们会优先评估扩区。
          </p>
          <motion.div
            className="mt-8 inline-flex"
            whileHover={{ y: -1 }}
            whileTap={{ scale: 0.97 }}
          >
            <a
              href="#lead"
              onClick={(e) => onHashLinkClick(e, "lead", { leadRole: "owner" })}
              className="inline-flex h-11 items-center justify-center rounded-full bg-primary px-5 text-sm font-semibold text-primary-foreground shadow-sm transition hover:bg-primary/90 active:scale-[0.97] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
            >
              申请开通我的小区
            </a>
          </motion.div>
        </Reveal>

        <Reveal delay={0.1}>
          <div className="overflow-hidden rounded-3xl border border-border/70 bg-card">
            <div className="relative h-44 min-h-[11rem] sm:h-52">
              <Image
                src={PET_IMAGES.dogLiving}
                alt="社区家庭中的狗狗"
                fill
                className="object-cover object-[center_30%]"
                sizes="(max-width: 1024px) 100vw, 420px"
              />
              <div className="absolute inset-0 bg-gradient-to-t from-card via-transparent to-transparent" />
            </div>
            <div className="relative -mt-6 px-5 pb-6 sm:px-6 sm:pb-8">
              <h3 className="text-sm font-semibold text-foreground">
                已开通示例区域
              </h3>
              <ul className="mt-4 divide-y divide-border/70 border-y border-border/70">
                {areas.map((area, index) => (
                  <motion.li
                    key={area}
                    initial={reduceMotion ? false : { x: 10 }}
                    whileInView={{ x: 0 }}
                    viewport={viewport}
                    transition={{
                      duration: 0.4,
                      ease: easeOut,
                      delay: 0.06 * index,
                    }}
                    className="flex items-center justify-between gap-4 py-3.5"
                  >
                    <span className="font-medium text-foreground">{area}</span>
                    <span className="text-xs font-medium text-trust">
                      已开通
                    </span>
                  </motion.li>
                ))}
              </ul>
              <p className="mt-5 text-xs leading-relaxed text-muted-foreground">
                当前服务为非医疗室内照护：喂食、换水、清理猫砂/生活区、拍照反馈。不含遛狗、喂药与夜间入户。
              </p>
            </div>
          </div>
        </Reveal>
      </div>
    </section>
  );
}
