import { Calendar } from "lucide-react";
import { formatShortDate } from "../../lib/utils";

interface DateBadgeProps {
  date: string;
}

export const DateBadge = ({ date }: DateBadgeProps) => {
  return (
    <div className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium text-muted-foreground bg-muted/80 backdrop-blur-md border border-border/40 shadow-xs select-none transition-all hover:bg-muted">
      <Calendar className="w-3 h-3 text-primary" />
      <span>{date}</span>
      <span className="text-[10px] opacity-70">({formatShortDate(date)})</span>
    </div>
  );
};

export default DateBadge;
