"use client";

import Image from "next/image";
import { onHashLinkClick } from "@/lib/scroll-to-id";

export function SiteFooter() {
  return (
    <footer className="border-t border-border/70 bg-background py-16">
      <div className="mx-auto flex max-w-6xl flex-col gap-8 px-4 sm:flex-row sm:items-end sm:justify-between sm:px-6">
        <div>
          <div className="flex items-center gap-2.5">
            <Image
              src="/images/logo.png"
              alt="喂宠到家"
              width={32}
              height={32}
              className="size-8 rounded-xl object-cover"
            />
            <strong className="font-semibold text-foreground">喂宠到家</strong>
          </div>
          <p className="mt-4 max-w-sm text-sm leading-relaxed text-muted-foreground">
            社区上门喂宠区域竞价平台。试点运营中，服务范围以开通小区为准。
          </p>
        </div>
        <div className="text-sm text-muted-foreground">
          <a
            href="#lead"
            onClick={(e) => onHashLinkClick(e, "lead")}
            className="rounded-md font-medium transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
          >
            申请试点开通
          </a>
          <span className="mx-2 text-border">·</span>
          <a
            href="#faq"
            onClick={(e) => onHashLinkClick(e, "faq")}
            className="rounded-md font-medium transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
          >
            常见问题
          </a>
          <p className="mt-4 text-xs">© 2026 喂宠到家</p>
        </div>
      </div>
    </footer>
  );
}
