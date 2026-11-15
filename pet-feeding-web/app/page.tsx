import { InstallPrompt, PushNotificationManager } from "@/components/pwa";

export default function Home() {
  return (
    <div className="flex flex-1 flex-col items-center justify-center bg-background px-6 py-16 font-sans">
      <main className="flex w-full max-w-md flex-col gap-10">
        <div className="flex flex-col gap-2">
          <h1 className="text-3xl font-semibold tracking-tight text-foreground">
            Pet Feeding
          </h1>
          <p className="text-muted-foreground">
            Install the app and enable notifications to stay on top of feeding
            schedules.
          </p>
        </div>

        <InstallPrompt />
        <PushNotificationManager />
      </main>
    </div>
  );
}
