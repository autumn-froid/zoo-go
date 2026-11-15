export type LeadRole = "owner" | "student";

export type CreateLeadPayload = {
  role: LeadRole;
  name: string;
  phone: string;
  community?: string;
  note?: string;
  source?: string;
};

export type CreateLeadResult = {
  id: string;
  status: string;
  message: string;
};

type ApiErrorBody = {
  error?: {
    code?: string;
    message?: string;
  };
};

function apiBaseUrl() {
  const configured = process.env.NEXT_PUBLIC_API_BASE_URL;
  if (configured !== undefined) {
    // Empty string = same-origin (Caddy proxies /v1 to the Go API).
    return configured.replace(/\/$/, "");
  }
  return "http://localhost:8080";
}

export async function createPilotLead(
  payload: CreateLeadPayload,
): Promise<CreateLeadResult> {
  const response = await fetch(`${apiBaseUrl()}/v1/leads`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      role: payload.role,
      name: payload.name.trim(),
      phone: payload.phone.trim(),
      community: payload.community?.trim() || "",
      note: payload.note?.trim() || "",
      source: payload.source || "website",
    }),
  });

  const body = (await response.json().catch(() => ({}))) as ApiErrorBody & {
    data?: CreateLeadResult;
  };

  if (!response.ok || !body.data) {
    throw new Error(body.error?.message || "提交失败，请稍后重试");
  }

  return body.data;
}
