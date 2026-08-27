import { Calendar } from "lucide-react";
import { formatWeekday } from "../../lib/utils";

interface DateBadgeProps {
  date: string;
}

export const DateBadge = ({ date }: DateBadgeProps) => {
  const weekday = formatWeekday(date);

  return (
    <div className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium text-muted-foreground bg-muted/80 backdrop-blur-md border border-border/40 shadow-xs select-none transition-all hover:bg-muted">
      <Calendar className="w-3.5 h-3.5 text-primary" />
      <span>{date}</span>
      {weekday && <span className="text-[11px] opacity-80">{weekday}</span>}
    </div>
  );
};

export default DateBadge;
