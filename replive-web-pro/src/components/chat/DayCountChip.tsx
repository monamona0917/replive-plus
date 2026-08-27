import { Heart } from "lucide-react";
import { memo } from "react";
import { getDayCountStyle } from "../../utils/fandom-limit";

interface DayCountChipProps {
  dayCount: number;
  className?: string;
}

export const DayCountChip = memo(({ dayCount, className = "" }: DayCountChipProps) => {
  if (!dayCount || dayCount < 1) return null;

  const style = getDayCountStyle(dayCount);

  return (
    <div
      className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-semibold select-none shadow-2xs transition-all animate-in zoom-in-90 duration-300 ${className}`}
      style={{
        color: style.color,
        backgroundColor: style.bgColor,
        border: `1px solid ${style.borderColor}`,
      }}
    >
      <Heart
        style={{
          width: `${style.heartSize}px`,
          height: `${style.heartSize}px`,
          fill: style.color,
          color: style.color,
        }}
        className="shrink-0 transition-transform duration-300 hover:scale-125"
      />
      <span className="font-mono text-[11px] leading-none font-bold tracking-tight">
        Day {dayCount}
      </span>
    </div>
  );
});

DayCountChip.displayName = "DayCountChip";

export default DayCountChip;
