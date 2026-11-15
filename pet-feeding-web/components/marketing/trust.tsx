"use client";

import Image from "next/image";
import { EyeOff, ShieldCheck, Snowflake } from "lucide-react";
import { motion, useReducedMotion } from "motion/react";
import { Reveal, easeOut, viewport } from "@/components/marketing/reveal";
import { PET_IMAGES } from "@/lib/images";

const items = [
  {
    icon: ShieldCheck,
    title: "实名 + 在校双重核验",
    detail:
      "服务者须年满 18 岁并完成实名与学籍核验。端上仅展示「已核验」，不展示证件信息。",
  },
  {
    icon: EyeOff,
    title: "成交前隐藏门牌",
    detail:
      "报价阶段只展示脱敏位置。付款后才向中选学生短期开放必要入户信息，访问可审计。",
  },
  {
    icon: Snowflake,
    title: "争议冻结结算",
    detail:
      "服务异常可上报，争议订单暂停自动结算，由人工客服介入处理，优先保障宠物与入户安全。",
  },
];

export function Trust() {
  const reduceMotion = useReducedMotion();

  return (
    <section id="trust" className="scroll-mt-20 bg-card py-24 sm:py-32">
      <div className="mx-auto max-w-6xl px-4 sm:px-6">
        <div className="grid items-center gap-12 lg:grid-cols-[0.9fr_1.1fr] lg:gap-16">
          <Reveal>
            <div className="relative aspect-[5/6] min-h-[16rem] overflow-hidden rounded-3xl">
              <Image
                src={PET_IMAGES.catWindow}
                alt="安静待在家中的猫咪"
                fill
                className="object-cover object-[center_20%]"
                sizes="(max-width: 1024px) 90vw, 380px"
              />
              <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/50 to-transparent px-5 py-5">
                <p className="text-sm font-medium text-white">
                  毛孩子在家等你回来
                </p>
              </div>
            </div>
          </Reveal>

          <div>
            <Reveal className="max-w-xl">
              <h2 className="text-3xl font-bold tracking-tight text-foreground sm:text-4xl">
                入户服务，先把信任做实
              </h2>
              <p className="mt-4 max-w-prose text-muted-foreground leading-relaxed">
                平台不做医疗机构，也不虚称兽医资质。我们把规则写清楚：谁能上门、何时开门牌、异常如何处理。
              </p>
            </Reveal>
            <div className="mt-10 divide-y divide-border/70 border-y border-border/70">
              {items.map((item, index) => (
                <motion.article
                  key={item.title}
                  initial={reduceMotion ? false : { y: 14 }}
                  whileInView={{ y: 0 }}
                  viewport={viewport}
                  transition={{
                    duration: 0.45,
                    ease: easeOut,
                    delay: 0.06 * index,
                  }}
                  className="flex gap-4 py-6 first:pt-6 last:pb-6"
                >
                  <span className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-trust-soft text-trust">
                    <item.icon className="size-5" aria-hidden />
                  </span>
                  <div className="min-w-0">
                    <h3 className="text-base font-semibold text-foreground">
                      {item.title}
                    </h3>
                    <p className="mt-1.5 text-sm leading-relaxed text-muted-foreground">
                      {item.detail}
                    </p>
                  </div>
                </motion.article>
              ))}
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
