"use client";

import Image from "next/image";
import { useEffect, useState } from "react";
import { onHashLinkClick } from "@/lib/scroll-to-id";
import { cn } from "@/lib/utils";

const links = [
  { href: "#how", label: "如何运作" },
  { href: "#trust", label: "安全保障" },
  { href: "#coverage", label: "服务范围" },
  { href: "#students", label: "学生加入" },
];

export function SiteHeader() {
  const [elevated, setElevated] = useState(false);

  useEffect(() => {
    const onScroll = () => setElevated(window.scrollY > 12);
    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  return (
    <header
      className={cn(
        "sticky top-0 z-40 border-b bg-background/90 backdrop-blur-md transition-[border-color,box-shadow] duration-200",
        elevated
          ? "border-border/95 shadow-[0_8px_24px_rgba(31,41,55,0.06)]"
          : "border-border/45 shadow-none",
      )}
    >
      <div className="mx-auto flex h-16 max-w-6xl items-center justify-between gap-3 px-4 sm:px-6">
        <a
          href="#top"
          onClick={(e) => onHashLinkClick(e, "top")}
          className="flex items-center gap-2.5 rounded-lg outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
        >
          <Image
            src="/images/logo.png"
            alt="喂宠到家"
            width={36}
            height={36}
            className="size-9 rounded-xl object-cover"
            priority
          />
          <span className="text-lg font-semibold tracking-tight text-foreground">
            喂宠到家
          </span>
        </a>
        <nav className="hidden items-center gap-8 text-sm font-medium text-muted-foreground md:flex">
          {links.map((link) => (
            <a
              key={link.href}
              href={link.href}
              onClick={(e) => onHashLinkClick(e, link.href.slice(1))}
              className="rounded-md underline-offset-4 transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
            >
              {link.label}
            </a>
          ))}
        </nav>
        <a
          href="#lead"
          onClick={(e) => onHashLinkClick(e, "lead", { leadRole: "owner" })}
          className="inline-flex h-10 shrink-0 items-center justify-center rounded-full bg-primary px-4 text-sm font-semibold text-primary-foreground shadow-sm transition hover:bg-primary/90 active:scale-[0.97] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
        >
          申请试点开通
        </a>
      </div>
    </header>
  );
}
