"use client";

import { useEffect, useState, useTransition, type FormEvent } from "react";
import { CheckCircle2, LoaderCircle } from "lucide-react";
import Image from "next/image";
import { AnimatePresence, motion } from "motion/react";
import { createPilotLead, type LeadRole } from "@/lib/api";
import { Reveal, easeOut } from "@/components/marketing/reveal";
import { PET_IMAGES } from "@/lib/images";
import { LEAD_ROLE_EVENT } from "@/lib/scroll-to-id";
import { cn } from "@/lib/utils";

type FormState = {
  role: LeadRole;
  name: string;
  phone: string;
  community: string;
  note: string;
};

const initialState: FormState = {
  role: "owner",
  name: "",
  phone: "",
  community: "",
  note: "",
};

export function LeadForm({
  defaultRole = "owner",
}: {
  defaultRole?: LeadRole;
}) {
  const [form, setForm] = useState<FormState>({
    ...initialState,
    role: defaultRole,
  });
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [pending, startTransition] = useTransition();

  function update<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((prev) => ({ ...prev, [key]: value }));
  }

  useEffect(() => {
    const onRole = (event: Event) => {
      const role = (event as CustomEvent<LeadRole>).detail;
      if (role === "owner" || role === "student") {
        update("role", role);
        setSuccess("");
      }
    };
    window.addEventListener(LEAD_ROLE_EVENT, onRole);
    return () => window.removeEventListener(LEAD_ROLE_EVENT, onRole);
  }, []);

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setSuccess("");
    startTransition(async () => {
      try {
        const result = await createPilotLead({
          role: form.role,
          name: form.name,
          phone: form.phone,
          community: form.community,
          note: form.note,
          source: "website",
        });
        setSuccess(result.message || "已收到申请，运营将尽快与您联系");
        setForm({ ...initialState, role: form.role });
      } catch (err) {
        setError(err instanceof Error ? err.message : "提交失败，请稍后重试");
      }
    });
  }

  return (
    <AnimatePresence mode="wait" initial={false}>
      {success ? (
        <motion.div
          key="success"
          initial={{ opacity: 0, scale: 0.96 }}
          animate={{ opacity: 1, scale: 1 }}
          exit={{ opacity: 0, scale: 0.98 }}
          transition={{ duration: 0.35, ease: easeOut }}
          className="rounded-3xl border border-trust/20 bg-trust-soft/40 p-8 text-center"
        >
          <motion.div
            initial={{ scale: 0.6, opacity: 0 }}
            animate={{ scale: 1, opacity: 1 }}
            transition={{
              type: "spring",
              stiffness: 320,
              damping: 18,
              delay: 0.05,
            }}
          >
            <CheckCircle2 className="mx-auto size-10 text-trust" aria-hidden />
          </motion.div>
          <h3 className="mt-4 text-xl font-semibold text-foreground">
            提交成功
          </h3>
          <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
            {success}
          </p>
          <button
            type="button"
            className="mt-6 rounded-md text-sm font-semibold text-primary underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
            onClick={() => setSuccess("")}
          >
            再提交一条
          </button>
        </motion.div>
      ) : (
        <motion.form
          key="form"
          initial={{ opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: -6 }}
          transition={{ duration: 0.28, ease: easeOut }}
          onSubmit={onSubmit}
          className="space-y-5"
          noValidate
        >
          <fieldset>
            <legend className="text-sm font-medium text-foreground">我是</legend>
            <div className="mt-2 grid grid-cols-2 gap-2">
              {(
                [
                  ["owner", "宠物主人"],
                  ["student", "在校学生"],
                ] as const
              ).map(([value, label]) => (
                <button
                  key={value}
                  type="button"
                  onClick={() => update("role", value)}
                  aria-pressed={form.role === value}
                  className={cn(
                    "h-11 rounded-full border text-sm font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60",
                    form.role === value
                      ? "border-primary bg-brand-soft text-primary"
                      : "border-border bg-card text-muted-foreground hover:border-primary/40 hover:text-foreground",
                  )}
                >
                  {label}
                </button>
              ))}
            </div>
          </fieldset>

          <label className="block space-y-1.5">
            <span className="text-sm font-medium text-foreground">姓名</span>
            <input
              required
              name="name"
              value={form.name}
              onChange={(e) => update("name", e.target.value)}
              className="h-11 w-full rounded-xl border border-input bg-card px-3 text-base text-foreground outline-none transition focus:border-primary focus:ring-2 focus:ring-ring/40 sm:text-sm"
              placeholder="怎么称呼您"
              maxLength={40}
              autoComplete="name"
            />
          </label>

          <label className="block space-y-1.5">
            <span className="text-sm font-medium text-foreground">手机号</span>
            <input
              required
              name="phone"
              type="tel"
              inputMode="numeric"
              value={form.phone}
              onChange={(e) => update("phone", e.target.value)}
              className="h-11 w-full rounded-xl border border-input bg-card px-3 text-base text-foreground outline-none transition focus:border-primary focus:ring-2 focus:ring-ring/40 sm:text-sm"
              placeholder="用于运营回访"
              maxLength={20}
              autoComplete="tel"
            />
          </label>

          <label className="block space-y-1.5">
            <span className="text-sm font-medium text-foreground">
              {form.role === "student" ? "学校 / 常驻区域" : "小区名称"}
            </span>
            <input
              name="community"
              value={form.community}
              onChange={(e) => update("community", e.target.value)}
              className="h-11 w-full rounded-xl border border-input bg-card px-3 text-base text-foreground outline-none transition focus:border-primary focus:ring-2 focus:ring-ring/40 sm:text-sm"
              placeholder={
                form.role === "student"
                  ? "例如：中国农业大学 / 马连洼"
                  : "例如：朗润园"
              }
              maxLength={80}
            />
          </label>

          <label className="block space-y-1.5">
            <span className="text-sm font-medium text-foreground">
              备注（可选）
            </span>
            <textarea
              name="note"
              value={form.note}
              onChange={(e) => update("note", e.target.value)}
              rows={3}
              className="w-full resize-none rounded-xl border border-input bg-card px-3 py-2.5 text-base text-foreground outline-none transition focus:border-primary focus:ring-2 focus:ring-ring/40 sm:text-sm"
              placeholder="宠物类型、期望时段或其他说明"
              maxLength={500}
            />
          </label>

          {error ? (
            <p
              className="rounded-xl bg-destructive/10 px-3 py-2 text-sm text-destructive"
              role="alert"
            >
              {error}
            </p>
          ) : null}

          <motion.button
            type="submit"
            disabled={pending}
            whileTap={{ scale: pending ? 1 : 0.98 }}
            className="inline-flex h-12 w-full items-center justify-center gap-2 rounded-full bg-primary text-sm font-semibold text-primary-foreground shadow-sm transition hover:bg-primary/90 disabled:opacity-60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
          >
            {pending ? (
              <>
                <LoaderCircle className="size-4 animate-spin" aria-hidden />
                提交中…
              </>
            ) : form.role === "student" ? (
              "申请成为喂宠员"
            ) : (
              "申请试点开通"
            )}
          </motion.button>
          <p className="text-xs leading-relaxed text-muted-foreground">
            提交即表示你同意我们仅为试点开通与服务回访联系你。不会向第三方出售你的联系方式。
          </p>
        </motion.form>
      )}
    </AnimatePresence>
  );
}

export function LeadSection() {
  return (
    <section id="lead" className="scroll-mt-20 py-24 sm:py-32">
      <div className="mx-auto grid max-w-6xl gap-12 px-4 sm:px-6 lg:grid-cols-[1fr_0.95fr] lg:items-start lg:gap-16">
        <Reveal className="lg:sticky lg:top-24">
          <div className="mb-8 flex items-center -space-x-2">
            <div className="relative size-14 overflow-hidden rounded-xl ring-2 ring-background">
              <Image
                src={PET_IMAGES.leadDog}
                alt=""
                fill
                className="object-cover"
                sizes="56px"
              />
            </div>
            <div className="relative size-14 overflow-hidden rounded-xl ring-2 ring-background">
              <Image
                src={PET_IMAGES.leadCat}
                alt=""
                fill
                className="object-cover"
                sizes="56px"
              />
            </div>
            <div className="relative size-14 overflow-hidden rounded-xl ring-2 ring-background">
              <Image
                src={PET_IMAGES.leadKitten}
                alt=""
                fill
                className="object-cover"
                sizes="56px"
              />
            </div>
          </div>
          <h2 className="text-3xl font-bold tracking-tight text-foreground sm:text-4xl">
            留下联系方式，我们帮你开通
          </h2>
          <p className="mt-4 max-w-md text-muted-foreground leading-relaxed">
            马连洼试点扩区与学生招募优先处理官网留资。提交后运营会在工作日内联系，确认小区覆盖或引导完成小程序认证。
          </p>
          <ul className="mt-8 space-y-3 text-sm text-foreground">
            <li className="flex gap-2">
              <span className="mt-1.5 size-1.5 shrink-0 rounded-full bg-primary" />
              主人：申请开通小区 / 加入试点
            </li>
            <li className="flex gap-2">
              <span className="mt-1.5 size-1.5 shrink-0 rounded-full bg-trust" />
              学生：申请核验成为喂宠员
            </li>
            <li className="flex gap-2">
              <span className="mt-1.5 size-1.5 shrink-0 rounded-full bg-muted-foreground" />
              同一手机号 24 小时内请勿重复提交
            </li>
          </ul>
        </Reveal>
        <Reveal delay={0.1}>
          <div className="rounded-3xl border border-border/70 bg-card p-6 sm:p-8">
            <LeadForm />
          </div>
        </Reveal>
      </div>
    </section>
  );
}
