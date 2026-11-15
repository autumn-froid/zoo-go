/** Canonical site origin. Set NEXT_PUBLIC_SITE_URL when a real domain is ready. */
export function getSiteUrl(): string {
  const explicit = process.env.NEXT_PUBLIC_SITE_URL?.trim();
  if (explicit) {
    return explicit.replace(/\/$/, "");
  }

  const vercel = process.env.VERCEL_URL?.trim();
  if (vercel) {
    return `https://${vercel.replace(/^https?:\/\//, "").replace(/\/$/, "")}`;
  }

  return "http://localhost:3000";
}

export const SITE_NAME = "喂宠到家";

export const SITE_TITLE = "喂宠到家｜上门喂宠竞价";

export const SITE_DESCRIPTION =
  "你设预算上限，实名核验通过的宠护师来报价，你来选人。实名核验、成交前隐藏门牌、服务凭证验收。覆盖各省各市已开通小区。";

export const SITE_OG_DESCRIPTION =
  "出门在外，毛孩子有人管。社区上门喂宠反向竞价平台，申请试点开通。";
