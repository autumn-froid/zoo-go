"use client";

import Image from "next/image";
import { MapPin } from "lucide-react";
import { motion, useReducedMotion } from "motion/react";
import { easeOut } from "@/components/marketing/reveal";
import { PET_IMAGES } from "@/lib/images";
import { onHashLinkClick } from "@/lib/scroll-to-id";

export function Hero() {
  const reduceMotion = useReducedMotion();

  return (
    <section id="top" className="relative isolate overflow-hidden">
      <div className="relative h-[42vh] min-h-[260px] max-h-[400px] w-full overflow-hidden md:absolute md:inset-0 md:h-auto md:min-h-0 md:max-h-none">
        <motion.div
          className="absolute inset-0"
          initial={reduceMotion ? false : { scale: 1.06 }}
          animate={{ scale: 1 }}
          transition={{ duration: 1.2, ease: easeOut }}
        >
          <Image
            src={PET_IMAGES.hero}
            alt="客厅里的金毛与猫咪，温馨居家陪伴"
            fill
            priority
            sizes="100vw"
            className="object-cover object-[72%_45%] sm:object-[75%_40%] md:object-[78%_42%]"
          />
        </motion.div>
        <div className="absolute inset-0 bg-gradient-to-t from-background via-background/25 to-transparent md:hidden" />
        <div className="absolute inset-0 hidden bg-gradient-to-r from-background via-background/88 to-background/12 md:block" />
        <div className="absolute inset-0 hidden bg-gradient-to-t from-background/65 via-transparent to-background/25 md:block" />
      </div>

      <div className="relative mx-auto flex max-w-6xl flex-col justify-center px-4 pb-12 pt-6 sm:px-6 sm:pb-16 sm:pt-8 md:min-h-[min(88vh,820px)] md:py-24">
        <div className="max-w-xl">
          <p className="inline-flex items-center gap-1.5 text-sm text-muted-foreground">
            <MapPin className="size-3.5 shrink-0" aria-hidden />
            各省各市已开通小区可发单
          </p>
          <h1 className="mt-4 text-4xl font-bold tracking-tight text-foreground sm:mt-6 sm:text-5xl lg:text-6xl">
            喂宠到家
          </h1>
          <p className="mt-3 text-xl font-semibold tracking-tight text-balance text-foreground sm:mt-4 sm:text-2xl lg:text-3xl">
            出门在外，
            <br />
            毛孩子有人管
          </p>
          <p className="mt-4 max-w-md text-base leading-relaxed text-muted-foreground sm:mt-6 sm:text-lg sm:leading-relaxed">
            上门喂宠竞价：你设预算上限，实名核验通过的宠护师来报价，你来选人。成交前隐藏门牌，服务后照片验收。
          </p>
          <div className="mt-7 flex flex-wrap items-center gap-3 sm:mt-8 md:mt-10">
            <a
              href="#lead"
              onClick={(e) => onHashLinkClick(e, "lead", { leadRole: "owner" })}
              className="inline-flex h-12 items-center justify-center rounded-full bg-primary px-6 text-sm font-semibold text-primary-foreground shadow-sm transition hover:bg-primary/90 active:scale-[0.97] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
            >
              申请试点开通
            </a>
            <a
              href="#how"
              onClick={(e) => onHashLinkClick(e, "how")}
              className="inline-flex h-12 items-center justify-center rounded-full border border-border bg-card/85 px-6 text-sm font-semibold text-foreground backdrop-blur transition hover:bg-card active:scale-[0.97] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
            >
              了解如何运作
            </a>
          </div>
        </div>
      </div>
    </section>
  );
}
