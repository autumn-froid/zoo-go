"use server";

import * as webpush from "web-push";

webpush.setVapidDetails(
  "mailto:hello@petfeeding.local",
  process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY!,
  process.env.VAPID_PRIVATE_KEY!,
);

type PushSubscriptionJSON = {
  endpoint: string;
  keys: {
    p256dh: string;
    auth: string;
  };
};

let subscription: PushSubscriptionJSON | null = null;

export async function subscribeUser(sub: PushSubscriptionJSON) {
  subscription = sub;
  // In production, persist this in a database.
  return { success: true };
}

export async function unsubscribeUser() {
  subscription = null;
  // In production, remove this from a database.
  return { success: true };
}

export async function sendNotification(message: string) {
  if (!subscription) {
    throw new Error("No subscription available");
  }

  try {
    await webpush.sendNotification(
      subscription,
      JSON.stringify({
        title: "Pet Feeding",
        body: message,
        icon: "/icon.png",
      }),
    );
    return { success: true };
  } catch (error) {
    console.error("Error sending push notification:", error);
    return { success: false, error: "Failed to send notification" };
  }
}
