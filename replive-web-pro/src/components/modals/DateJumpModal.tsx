import {
  addMonths,
  eachDayOfInterval,
  endOfMonth,
  endOfWeek,
  format,
  isSameMonth,
  parse,
  startOfMonth,
  startOfWeek,
  subMonths,
} from "date-fns";
import {
  Calendar as CalendarIcon,
  ChevronLeft,
  ChevronRight,
  Loader2,
  X,
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { cn, currentLocalCalendarDate } from "../../lib/utils";
import useChatStore, { roomKey } from "../../stores/chat-store";

export const DateJumpModal = () => {
  const dateJumpModalOpen = useChatStore((s) => s.dateJumpModalOpen);
  const setDateJumpModalOpen = useChatStore((s) => s.setDateJumpModalOpen);
  const jumpToDate = useChatStore((s) => s.jumpToDate);
  const selectedRoom = useChatStore((s) => s.selectedRoom);
  const availableDatesByRoom = useChatStore((s) => s.availableDatesByRoom);
  const isLoadingDates = useChatStore((s) => s.isLoadingDates);

  const [currentMonth, setCurrentMonth] = useState<Date>(currentLocalCalendarDate);

  const availableDates = useMemo(() => {
    if (!selectedRoom) return [];
    const key = roomKey(selectedRoom);
    return availableDatesByRoom[key] || [];
  }, [selectedRoom, availableDatesByRoom]);

  // 当打开弹窗或获取到日期列表时，默认将日历视图定位到最新有记录的那个月份
  useEffect(() => {
    if (dateJumpModalOpen && availableDates.length > 0) {
      const latestDateStr = availableDates[availableDates.length - 1];
      try {
        const parsed = parse(latestDateStr, "yyyy-MM-dd", new Date());
        if (!isNaN(parsed.getTime())) {
          setCurrentMonth(parsed);
        }
      } catch {
        // fallback
      }
    }
  }, [dateJumpModalOpen, availableDates]);

  if (!dateJumpModalOpen) return null;

  // 生成当前月份的完整日历网格（含前后补齐周）
  const monthStart = startOfMonth(currentMonth);
  const monthEnd = endOfMonth(monthStart);
  const startDate = startOfWeek(monthStart, { weekStartsOn: 0 });
  const endDate = endOfWeek(monthEnd, { weekStartsOn: 0 });
  const daysGrid = eachDayOfInterval({ start: startDate, end: endDate });

  const handlePrevMonth = () => setCurrentMonth((prev) => subMonths(prev, 1));
  const handleNextMonth = () => setCurrentMonth((prev) => addMonths(prev, 1));

  const handleJumpToLatestMonth = () => {
    if (availableDates.length > 0) {
      const latestDateStr = availableDates[availableDates.length - 1];
      try {
        const parsed = parse(latestDateStr, "yyyy-MM-dd", new Date());
        setCurrentMonth(parsed);
      } catch {
        setCurrentMonth(currentLocalCalendarDate());
      }
    } else {
      setCurrentMonth(currentLocalCalendarDate());
    }
  };

  const handleSelectDate = (dateStr: string) => {
    void jumpToDate(dateStr);
    setDateJumpModalOpen(false);
  };

  const weekdays = ["日", "一", "二", "三", "四", "五", "六"];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm animate-in fade-in duration-200 select-none">
      <div className="w-full max-w-sm bg-card border border-border rounded-2xl shadow-2xl p-5 relative animate-in zoom-in-95 duration-200">
        {/* Close Button */}
        <button
          type="button"
          onClick={() => setDateJumpModalOpen(false)}
          className="absolute right-4 top-4 p-1.5 rounded-lg text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
        >
          <X className="w-4 h-4" />
        </button>

        {/* Modal Header */}
        <div className="flex items-center gap-2.5 mb-4">
          <div className="p-2 rounded-xl bg-primary/15 text-primary">
            <CalendarIcon className="w-4 h-4" />
          </div>
          <div className="flex items-center gap-2">
            <h2 className="text-base font-bold text-foreground">日期跳转</h2>
            {isLoadingDates && (
              <Loader2 className="w-3.5 h-3.5 animate-spin text-muted-foreground" />
            )}
          </div>
        </div>

        {/* Calendar Card View */}
        <div className="p-3.5 rounded-xl bg-muted/30 border border-border/60">
          {/* Month Navigation */}
          <div className="flex items-center justify-between mb-3 px-1">
            <button
              type="button"
              onClick={handlePrevMonth}
              className="p-1 rounded-lg text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
              title="上一个月"
            >
              <ChevronLeft className="w-4 h-4" />
            </button>

            <div className="flex items-center gap-2">
              <span className="text-sm font-bold text-foreground font-mono">
                {format(currentMonth, "yyyy年 MM月")}
              </span>
              <button
                type="button"
                onClick={handleJumpToLatestMonth}
                className="px-2 py-0.5 text-[10px] rounded-md bg-muted text-muted-foreground hover:text-foreground hover:bg-muted/80 transition-colors font-medium"
              >
                最新
              </button>
            </div>

            <button
              type="button"
              onClick={handleNextMonth}
              className="p-1 rounded-lg text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
              title="下一个月"
            >
              <ChevronRight className="w-4 h-4" />
            </button>
          </div>

          {/* Weekday Header */}
          <div className="grid grid-cols-7 gap-1 text-center mb-1.5">
            {weekdays.map((w, idx) => (
              <span
                key={w}
                className={cn(
                  "text-[11px] font-semibold py-1",
                  idx === 0 || idx === 6
                    ? "text-muted-foreground/60"
                    : "text-muted-foreground",
                )}
              >
                {w}
              </span>
            ))}
          </div>

          {/* Days Grid */}
          <div className="grid grid-cols-7 gap-1">
            {daysGrid.map((day) => {
              const dateStr = format(day, "yyyy-MM-dd");
              const isCurrentMonthDay = isSameMonth(day, currentMonth);
              const hasChat = availableDates.includes(dateStr);

              return (
                <button
                  type="button"
                  key={dateStr}
                  onClick={() => hasChat && handleSelectDate(dateStr)}
                  disabled={!hasChat}
                  className={cn(
                    "relative aspect-square flex flex-col items-center justify-center rounded-xl text-xs transition-all font-mono",
                    // 非当月日期置灰
                    !isCurrentMonthDay && "opacity-20",
                    // 有聊天记录：高亮且可点击
                    hasChat &&
                      isCurrentMonthDay &&
                      "bg-primary/15 text-primary border border-primary/40 font-bold hover:bg-primary hover:text-primary-foreground active:scale-95 shadow-xs cursor-pointer",
                    // 无聊天记录：置灰且禁用
                    !hasChat &&
                      "opacity-20 text-muted-foreground cursor-not-allowed pointer-events-none border border-transparent",
                  )}
                  title={
                    hasChat
                      ? `${dateStr} (点击跳转到该日消息)`
                      : `${dateStr} (无聊天记录)`
                  }
                >
                  <span>{format(day, "d")}</span>
                  {hasChat && isCurrentMonthDay && (
                    <span className="w-1 h-1 rounded-full bg-primary mt-0.5" />
                  )}
                </button>
              );
            })}
          </div>
        </div>

        {/* Footer info */}
        <div className="mt-3 flex items-center justify-between text-[11px] text-muted-foreground px-1">
          <div className="flex items-center gap-1.5">
            <span className="w-2 h-2 rounded-full bg-primary" />
            <span>高亮日期包含历史消息记录</span>
          </div>
          <span className="font-mono">共 {availableDates.length} 天记录</span>
        </div>
      </div>
    </div>
  );
};

export default DateJumpModal;
