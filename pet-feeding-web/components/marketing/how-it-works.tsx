"use client";

import Image from "next/image";
import { Camera, ClipboardEdit, CreditCard, Gavel } from "lucide-react";
import { motion, useReducedMotion } from "motion/react";
import { Reveal, easeOut, viewport } from "@/components/marketing/reveal";
import { PET_IMAGES } from "@/lib/images";

const steps = [
  {
    icon: ClipboardEdit,
    title: "发布任务",
    detail: "选宠物与时段，勾选照护清单，设预算上限和竞价截止。",
  },
  {
    icon: Gavel,
    title: "宠护师报价",
    detail: "附近实名核验通过的宠护师保密报价，你可看完成单量与评价后比较。",
  },
  {
    icon: CreditCard,
    title: "选人支付",
    detail: "择优确认中选宠护师并付款，门牌信息才对其短期开放。",
  },
  {
    icon: Camera,
    title: "凭证验收",
    detail: "到场签到、按清单照护并上传照片，你验收后结算。",
  },
];

export function HowItWorks() {
  const reduceMotion = useReducedMotion();

  return (
    <section
      id="how"
      className="scroll-mt-20 border-t border-border/60 py-24 sm:py-32"
    >
      <div className="mx-auto max-w-6xl px-4 sm:px-6">
        <Reveal className="max-w-2xl">
          <h2 className="text-3xl font-bold tracking-tight text-foreground sm:text-4xl">
            反向竞价，你来做主
          </h2>
          <p className="mt-4 max-w-prose text-muted-foreground leading-relaxed">
            不是宠护师挂价等人挑。你设价格上限，实名核验通过的宠护师来报价，成交与否由你决定。
          </p>
        </Reveal>

        <Reveal delay={0.08} className="mt-12">
          <div className="relative overflow-hidden rounded-3xl">
            <div className="relative aspect-[16/9] sm:aspect-[21/9]">
              <Image
                src={PET_IMAGES.feedingMoment}
                alt="在家准备喂食的柯基"
                fill
                className="object-cover object-[center_42%]"
                sizes="(max-width: 1152px) 100vw, 1152px"
              />
              <div className="absolute inset-0 bg-gradient-to-t from-foreground/70 via-foreground/15 to-transparent" />
            </div>
            <div className="absolute inset-x-0 bottom-0 flex flex-col gap-2 p-5 sm:flex-row sm:items-end sm:justify-between sm:gap-4 sm:p-8">
              <p className="max-w-md text-lg font-semibold tracking-tight text-white sm:text-xl">
                出门在外，毛孩子照样被好好照顾
              </p>
              <p className="text-sm text-white/80">
                设预算 → 收报价 → 你选人 → 看凭证
              </p>
            </div>
          </div>
        </Reveal>

        <ol className="relative mt-16 grid gap-0 sm:grid-cols-2 lg:grid-cols-4 lg:gap-6">
          <div
            aria-hidden
            className="pointer-events-none absolute top-[1.375rem] right-[12.5%] left-[12.5%] hidden h-px bg-border lg:block"
          />

          {steps.map((step, index) => (
            <motion.li
              key={step.title}
              initial={reduceMotion ? false : { y: 12 }}
              whileInView={{ y: 0 }}
              viewport={viewport}
              transition={{
                duration: 0.4,
                ease: easeOut,
                delay: 0.06 * index,
              }}
              className="relative border-b border-border/70 py-6 last:border-b-0 sm:border-b-0 sm:py-0 lg:px-1"
            >
              <div className="flex gap-4 lg:flex-col lg:gap-4">
                <span className="relative z-10 flex size-11 shrink-0 items-center justify-center rounded-full border border-border bg-card text-sm font-semibold text-foreground shadow-sm">
                  {String(index + 1).padStart(2, "0")}
                </span>
                <div className="min-w-0 pt-0.5">
                  <div className="flex items-center gap-2">
                    <step.icon
                      className="size-4 shrink-0 text-muted-foreground"
                      aria-hidden
                    />
                    <h3 className="text-base font-semibold text-foreground sm:text-lg">
                      {step.title}
                    </h3>
                  </div>
                  <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
                    {step.detail}
                  </p>
                </div>
              </div>
            </motion.li>
          ))}
        </ol>
      </div>
    </section>
  );
}
