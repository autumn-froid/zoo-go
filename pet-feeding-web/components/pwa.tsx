"use client";

import { useEffect, useState } from "react";

import { subscribeUser, unsubscribeUser, sendNotification } from "@/app/actions";
import { Button } from "@/components/ui/button";

function urlBase64ToUint8Array(base64String: string) {
  const padding = "=".repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, "+").replace(/_/g, "/");

  const rawData = window.atob(base64);
  const outputArray = new Uint8Array(rawData.length);

  for (let i = 0; i < rawData.length; ++i) {
    outputArray[i] = rawData.charCodeAt(i);
  }
  return outputArray;
}

export function PushNotificationManager() {
  const [isSupported, setIsSupported] = useState(false);
  const [subscription, setSubscription] = useState<PushSubscription | null>(
    null,
  );
  const [message, setMessage] = useState("");

  useEffect(() => {
    if ("serviceWorker" in navigator && "PushManager" in window) {
      setIsSupported(true);
      void registerServiceWorker();
    }
  }, []);

  async function registerServiceWorker() {
    const registration = await navigator.serviceWorker.register(
      new URL("../lib/service-worker.js", import.meta.url),
      {
        scope: "/",
        updateViaCache: "none",
      },
    );
    const sub = await registration.pushManager.getSubscription();
    setSubscription(sub);
  }

  async function subscribeToPush() {
    const registration = await navigator.serviceWorker.ready;
    const sub = await registration.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: urlBase64ToUint8Array(
        process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY!,
      ),
    });
    setSubscription(sub);
    const serializedSub = JSON.parse(JSON.stringify(sub));
    await subscribeUser(serializedSub);
  }

  async function unsubscribeFromPush() {
    await subscription?.unsubscribe();
    setSubscription(null);
    await unsubscribeUser();
  }

  async function sendTestNotification() {
    if (subscription) {
      await sendNotification(message);
      setMessage("");
    }
  }

  if (!isSupported) {
    return (
      <p className="text-sm text-muted-foreground">
        Push notifications are not supported in this browser.
      </p>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      <h3 className="text-base font-medium">Push Notifications</h3>
      {subscription ? (
        <>
          <p className="text-sm text-muted-foreground">
            You are subscribed to push notifications.
          </p>
          <Button variant="outline" onClick={() => void unsubscribeFromPush()}>
            Unsubscribe
          </Button>
          <input
            type="text"
            className="h-8 rounded-lg border border-border bg-background px-2.5 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
            placeholder="Enter notification message"
            value={message}
            onChange={(e) => setMessage(e.target.value)}
          />
          <Button onClick={() => void sendTestNotification()}>Send Test</Button>
        </>
      ) : (
        <>
          <p className="text-sm text-muted-foreground">
            You are not subscribed to push notifications.
          </p>
          <Button onClick={() => void subscribeToPush()}>Subscribe</Button>
        </>
      )}
    </div>
  );
}

export function InstallPrompt() {
  const [isIOS, setIsIOS] = useState(false);
  const [isStandalone, setIsStandalone] = useState(false);

  useEffect(() => {
    setIsIOS(
      /iPad|iPhone|iPod/.test(navigator.userAgent) &&
        !(window as Window & { MSStream?: unknown }).MSStream,
    );
    setIsStandalone(window.matchMedia("(display-mode: standalone)").matches);
  }, []);

  if (isStandalone) {
    return null;
  }

  return (
    <div className="flex flex-col gap-3">
      <h3 className="text-base font-medium">Install App</h3>
      <p className="text-sm text-muted-foreground">
        Install Pet Feeding for quick access from your home screen.
      </p>
      {isIOS && (
        <p className="text-sm text-muted-foreground">
          To install this app on your iOS device, tap the share button{" "}
          <span role="img" aria-label="share icon">
            ⎋
          </span>{" "}
          and then &quot;Add to Home Screen{" "}
          <span role="img" aria-label="plus icon">
            ➕
          </span>
          &quot;.
        </p>
      )}
    </div>
  );
}
