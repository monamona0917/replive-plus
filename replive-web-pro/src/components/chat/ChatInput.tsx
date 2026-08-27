import { SendHorizontal } from "lucide-react";
import { type ChangeEvent, type KeyboardEvent, useRef, useState } from "react";
import { cn } from "../../lib/utils";
import useChatStore from "../../stores/chat-store";
import {
  getMaxCharacterLimit,
  getUtf16Length,
} from "../../utils/fandom-limit";

export const ChatInput = () => {
  const selectedRoom = useChatStore((s) => s.selectedRoom);
  const userProfile = useChatStore((s) => s.userProfile);
  const sendMessage = useChatStore((s) => s.sendMessage);
  const setError = useChatStore((s) => s.setError);

  const [text, setText] = useState("");
  const [isSending, setIsSending] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const isPrimeChat = selectedRoom?.category === "prime";
  const isOffline = userProfile?.offlineMode === true;
  const isSendDisabled =
    !userProfile ||
    isOffline ||
    isPrimeChat ||
    userProfile.sendChatEnabled === false;

  const isFandom = selectedRoom?.category === "fandom";
  const maxLimit = isFandom
    ? getMaxCharacterLimit(selectedRoom?.dayCount)
    : 1000;
  const currentLength = getUtf16Length(text);
  const isOverLimit = isFandom && currentLength > maxLimit;

  const handleTextChange = (e: ChangeEvent<HTMLTextAreaElement>) => {
    setText(e.target.value);
    const target = e.target;
    target.style.height = "auto";
    target.style.height = `${Math.min(target.scrollHeight, 120)}px`;
  };

  const handleSend = async () => {
    const trimmed = text.trim();
    if (!trimmed || isSending || !selectedRoom || isSendDisabled) return;

    if (isFandom && getUtf16Length(trimmed) > maxLimit) {
      setError(
        `消息长度超出上限（已输入 ${getUtf16Length(trimmed)} 字，最大允许 ${maxLimit} 字）`,
      );
      return;
    }

    setIsSending(true);
    try {
      await sendMessage(trimmed);
      setText("");
      if (textareaRef.current) {
        textareaRef.current.style.height = "auto";
        textareaRef.current.focus();
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "发送失败");
    } finally {
      setIsSending(false);
    }
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey && !e.ctrlKey) {
      e.preventDefault();
      void handleSend();
    }
  };

  if (isSendDisabled) {
    return (
      <footer className="border-t border-border/60 bg-card/60 backdrop-blur-md px-4 py-3 select-none">
        <div className="flex items-center justify-center p-2 rounded-xl bg-muted/40 border border-border/40 text-xs text-muted-foreground">
          <span>
            {isPrimeChat
              ? "Prime Chat 仅支持查看，暂不支持发送消息。"
              : isOffline
                ? "将以离线模式运行，仅浏览本地聊天记录。"
                : "消息发送功能已在配置中禁用"}
          </span>
        </div>
      </footer>
    );
  }

  return (
    <footer className="border-t border-border/60 bg-background/80 backdrop-blur-lg px-3 sm:px-5 py-3">
      <div className="flex items-end gap-2 w-full max-w-none px-1 sm:px-4">
        <div
          className={cn(
            "relative flex-1 flex flex-col rounded-2xl bg-card border shadow-xs transition-all",
            isOverLimit
              ? "border-red-500/80 focus-within:border-red-500 focus-within:ring-2 focus-within:ring-red-500/20"
              : "border-border/80 focus-within:border-primary focus-within:ring-2 focus-within:ring-primary/20",
          )}
        >
          <textarea
            ref={textareaRef}
            rows={1}
            value={text}
            onChange={handleTextChange}
            onKeyDown={handleKeyDown}
            disabled={!selectedRoom || isSending}
            placeholder={
              selectedRoom
                ? `向 ${selectedRoom.displayName} 发送消息... (Enter 发送, Shift+Enter 换行)`
                : "请先从左侧选择一个聊天对象"
            }
            className="w-full min-h-[42px] max-h-[120px] resize-none bg-transparent px-3.5 pt-2.5 pb-1 text-sm outline-none placeholder:text-muted-foreground/60 disabled:opacity-50"
          />

          {isFandom && (
            <div className="flex justify-end items-center px-3.5 pb-1.5 text-[10.5px] font-mono select-none pointer-events-none">
              <span
                className={cn(
                  "transition-colors",
                  isOverLimit
                    ? "text-red-400 font-bold"
                    : currentLength === maxLimit
                      ? "text-amber-400 font-bold"
                      : currentLength > maxLimit * 0.8
                        ? "text-muted-foreground font-medium"
                        : "text-muted-foreground/50",
                )}
                title={`字数统计：已输入 ${currentLength} 字 / 最大上限 ${maxLimit} 字${isOverLimit ? `（超出 ${currentLength - maxLimit} 字）` : ""}`}
              >
                {currentLength} / {maxLimit}
              </span>
            </div>
          )}
        </div>

        <button
          type="button"
          onClick={() => void handleSend()}
          disabled={!selectedRoom || isSending || !text.trim() || isOverLimit}
          className={cn(
            "flex items-center justify-center w-10 h-10 rounded-xl bg-primary text-primary-foreground shadow-sm transition-all shrink-0 mb-1",
            !text.trim() || isSending || !selectedRoom || isOverLimit
              ? "opacity-40 cursor-not-allowed"
              : "hover:opacity-90 active:scale-95 shadow-primary/25",
          )}
          title={isOverLimit ? `字数超出上限（最大 ${maxLimit} 字）` : "发送消息"}
        >
          <SendHorizontal className="w-4 h-4" />
        </button>
      </div>
    </footer>
  );
};

export default ChatInput;
