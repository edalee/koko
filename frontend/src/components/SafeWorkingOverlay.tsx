import { Coffee, Moon } from "lucide-react";
import { useEffect, useState } from "react";

function formatCountdown(seconds: number): string {
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = seconds % 60;
  if (h > 0) return `${h}h ${m.toString().padStart(2, "0")}m`;
  if (m > 0) return `${m}m ${s.toString().padStart(2, "0")}s`;
  return `${s}s`;
}

function formatTime(date: Date): string {
  return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

interface QuietHoursOverlayProps {
  resumeTime: Date;
}

function QuietHoursOverlay({ resumeTime }: QuietHoursOverlayProps) {
  const [secondsLeft, setSecondsLeft] = useState(0);

  useEffect(() => {
    function update() {
      const diff = Math.max(0, Math.floor((resumeTime.getTime() - Date.now()) / 1000));
      setSecondsLeft(diff);
    }
    update();
    const id = setInterval(update, 1000);
    return () => clearInterval(id);
  }, [resumeTime]);

  return (
    <div className="fixed inset-0 z-[200] flex items-center justify-center bg-black/80 backdrop-blur-xl">
      <div className="flex flex-col items-center gap-6 text-center max-w-md px-8">
        <div className="size-20 rounded-full bg-indigo-500/20 flex items-center justify-center">
          <Moon className="size-10 text-indigo-400" />
        </div>
        <div>
          <h2 className="text-2xl font-semibold text-white mb-2">Time to rest</h2>
          <p className="text-sm text-white/60 leading-relaxed">
            Quiet hours are active. Take care of yourself — the code will be here when you get back.
          </p>
        </div>
        <div className="flex flex-col items-center gap-2">
          <span className="text-4xl font-mono font-light text-white/90 tabular-nums">
            {formatCountdown(secondsLeft)}
          </span>
          <span className="text-xs text-white/40">Resuming at {formatTime(resumeTime)}</span>
        </div>
      </div>
    </div>
  );
}

interface BreakOverlayProps {
  secondsLeft: number;
  totalSeconds: number;
  onSkip: () => void;
}

function BreakOverlay({ secondsLeft, totalSeconds, onSkip }: BreakOverlayProps) {
  const progress = totalSeconds > 0 ? 1 - secondsLeft / totalSeconds : 0;
  const circumference = 2 * Math.PI * 54;
  const strokeOffset = circumference * (1 - progress);

  return (
    <div className="fixed inset-0 z-[200] flex items-center justify-center bg-black/70 backdrop-blur-lg">
      <div className="flex flex-col items-center gap-6 text-center max-w-md px-8">
        <div className="relative size-32 flex items-center justify-center">
          {/* Progress ring */}
          <svg
            className="absolute inset-0 -rotate-90"
            viewBox="0 0 120 120"
            role="img"
            aria-label="Break timer progress"
          >
            <circle
              cx="60"
              cy="60"
              r="54"
              fill="none"
              stroke="rgba(255,255,255,0.08)"
              strokeWidth="4"
            />
            <circle
              cx="60"
              cy="60"
              r="54"
              fill="none"
              stroke="rgb(31, 242, 171)"
              strokeWidth="4"
              strokeLinecap="round"
              strokeDasharray={circumference}
              strokeDashoffset={strokeOffset}
              className="transition-[stroke-dashoffset] duration-1000 ease-linear"
            />
          </svg>
          <Coffee className="size-10 text-accent" />
        </div>
        <div>
          <h2 className="text-2xl font-semibold text-white mb-2">Take a break</h2>
          <p className="text-sm text-white/60 leading-relaxed">
            Stretch, hydrate, rest your eyes. You've earned it.
          </p>
        </div>
        <span className="text-4xl font-mono font-light text-white/90 tabular-nums">
          {formatCountdown(secondsLeft)}
        </span>
        <button
          type="button"
          onClick={onSkip}
          className="text-xs text-white/30 hover:text-white/50 transition-colors mt-4"
        >
          Skip this break
        </button>
      </div>
    </div>
  );
}

interface SafeWorkingOverlayProps {
  isQuietHours: boolean;
  quietResumeTime: Date | null;
  isBreakTime: boolean;
  breakSecondsLeft: number;
  breakTotalSeconds: number;
  onSkipBreak: () => void;
}

export default function SafeWorkingOverlay({
  isQuietHours,
  quietResumeTime,
  isBreakTime,
  breakSecondsLeft,
  breakTotalSeconds,
  onSkipBreak,
}: SafeWorkingOverlayProps) {
  // Quiet hours takes priority
  if (isQuietHours && quietResumeTime) {
    return <QuietHoursOverlay resumeTime={quietResumeTime} />;
  }

  if (isBreakTime) {
    return (
      <BreakOverlay
        secondsLeft={breakSecondsLeft}
        totalSeconds={breakTotalSeconds}
        onSkip={onSkipBreak}
      />
    );
  }

  return null;
}
