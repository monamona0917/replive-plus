import { Loader2 } from "lucide-react";
import type React from "react";
import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import useChatStore, { groupMessagesByDate, roomKey } from "../../stores/chat-store";
import DateBadge from "./DateBadge";
import MessageBubble from "./MessageBubble";

const TOP_LOAD_THRESHOLD = 50;
const SCROLL_UP_TOLERANCE = 1;

export const ChatList = () => {
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const messageContentRef = useRef<HTMLDivElement>(null);
  const isFetchingOlderRef = useRef(false);
  const keepAtBottomRef = useRef(false);
  const lastScrollTopRef = useRef(0);
  const [keepAtBottomOnRoomEntry, setKeepAtBottomOnRoomEntry] =
    useState(false);

  const selectedRoom = useChatStore((s) => s.selectedRoom);
  const userProfile = useChatStore((s) => s.userProfile);
  const messagesByRoom = useChatStore((s) => s.messagesByRoom);
  const hasMoreByRoom = useChatStore((s) => s.hasMoreByRoom);
  const hasNewerByRoom = useChatStore((s) => s.hasNewerByRoom);
  const isLoadingMessagesByRoom = useChatStore((s) => s.isLoadingMessagesByRoom);
  const isLoadingMore = useChatStore((s) => s.isLoadingMore);
  const isLoadingNewer = useChatStore((s) => s.isLoadingNewer);
  const jumpTargetMessageId = useChatStore((s) => s.jumpTargetMessageId);
  const scrollToBottomToken = useChatStore((s) => s.scrollToBottomToken);

  const loadOlderMessages = useChatStore((s) => s.loadOlderMessages);
  const loadNewerMessages = useChatStore((s) => s.loadNewerMessages);
  const clearJumpTarget = useChatStore((s) => s.clearJumpTarget);

  const activeKey = selectedRoom ? roomKey(selectedRoom) : "";
  const messageGroups = useMemo(
    () => groupMessagesByDate(activeKey ? messagesByRoom[activeKey] ?? [] : []),
    [activeKey, messagesByRoom],
  );
  const isLoadingMessages = activeKey
    ? (isLoadingMessagesByRoom[activeKey] ?? false)
    : false;
  const hasMoreHistory = selectedRoom ? (hasMoreByRoom[activeKey] ?? false) : false;
  const hasNewerMessages = selectedRoom
    ? (hasNewerByRoom[activeKey] ?? false)
    : false;

  // 切换房间时先贴住底部，直到用户主动向上浏览历史消息。
  useLayoutEffect(() => {
    const shouldKeepAtBottom = Boolean(activeKey);
    keepAtBottomRef.current = shouldKeepAtBottom;
    lastScrollTopRef.current = 0;
    setKeepAtBottomOnRoomEntry(shouldKeepAtBottom);
  }, [activeKey]);

  // 跳转位置优先于首次进入时的贴底逻辑。
  useLayoutEffect(() => {
    if (!jumpTargetMessageId) return;

    const target = document.getElementById(`msg-${jumpTargetMessageId}`);
    if (!target) return;

    keepAtBottomRef.current = false;
    setKeepAtBottomOnRoomEntry(false);
  }, [activeKey, jumpTargetMessageId, messageGroups]);

  // 首次进入房间后定位到最新消息。
  useEffect(() => {
    const container = scrollContainerRef.current;
    if (
      !container ||
      !activeKey ||
      !keepAtBottomRef.current ||
      jumpTargetMessageId ||
      hasNewerMessages ||
      isLoadingMessages
    ) {
      return;
    }

    container.scrollTop = container.scrollHeight;
    lastScrollTopRef.current = container.scrollTop;
  }, [
    activeKey,
    hasNewerMessages,
    isLoadingMessages,
    jumpTargetMessageId,
    keepAtBottomOnRoomEntry,
  ]);

  // 图片等媒体的固有尺寸在加载后会改变消息内容高度，继续维持首次进入时的底部锚点。
  useEffect(() => {
    const content = messageContentRef.current;
    if (
      !content ||
      !activeKey ||
      !keepAtBottomOnRoomEntry ||
      isLoadingMessages
    ) {
      return;
    }

    let animationFrameId = 0;
    const keepAtBottom = () => {
      if (
        !keepAtBottomRef.current ||
        jumpTargetMessageId ||
        hasNewerMessages ||
        animationFrameId
      ) {
        return;
      }

      animationFrameId = requestAnimationFrame(() => {
        animationFrameId = 0;
        const container = scrollContainerRef.current;
        if (
          !container ||
          !keepAtBottomRef.current ||
          jumpTargetMessageId ||
          hasNewerMessages
        ) {
          return;
        }

        container.scrollTop = container.scrollHeight;
        lastScrollTopRef.current = container.scrollTop;
      });
    };

    const observer = new ResizeObserver(keepAtBottom);
    observer.observe(content);
    keepAtBottom();

    return () => {
      observer.disconnect();
      if (animationFrameId) {
        cancelAnimationFrame(animationFrameId);
      }
    };
  }, [
    activeKey,
    hasNewerMessages,
    isLoadingMessages,
    jumpTargetMessageId,
    keepAtBottomOnRoomEntry,
  ]);

  // 目标消息精准定位与平滑居中
  useEffect(() => {
    if (!jumpTargetMessageId) return;

    const timer = setTimeout(() => {
      const el = document.getElementById(`msg-${jumpTargetMessageId}`);
      if (el) {
        el.scrollIntoView({ block: "center", behavior: "smooth" });
      }
      clearJumpTarget();
    }, 100);

    return () => clearTimeout(timer);
  }, [jumpTargetMessageId, clearJumpTarget]);

  // 手动请求滚动到底部
  useEffect(() => {
    const container = scrollContainerRef.current;
    if (!container || scrollToBottomToken === 0) return;

    requestAnimationFrame(() => {
      container.scrollTo({ top: container.scrollHeight, behavior: "smooth" });
    });
  }, [scrollToBottomToken]);

  // 滚动监听与向上防跳跃加载
  const handleScroll = (e: React.UIEvent<HTMLDivElement>) => {
    const target = e.currentTarget;

    if (
      keepAtBottomRef.current &&
      target.scrollTop < lastScrollTopRef.current - SCROLL_UP_TOLERANCE
    ) {
      keepAtBottomRef.current = false;
      setKeepAtBottomOnRoomEntry(false);
    }
    lastScrollTopRef.current = target.scrollTop;

    // 向上触顶加载更早历史
    if (
      target.scrollTop <= TOP_LOAD_THRESHOLD &&
      !isLoadingMore &&
      !isFetchingOlderRef.current &&
      hasMoreHistory &&
      selectedRoom
    ) {
      void triggerLoadMore(target);
      return;
    }

    // 向下触底加载更新历史
    if (
      hasNewerMessages &&
      !isLoadingNewer &&
      target.scrollHeight - target.scrollTop - target.clientHeight <=
        TOP_LOAD_THRESHOLD &&
      selectedRoom
    ) {
      void loadNewerMessages(selectedRoom);
    }
  };

  const triggerLoadMore = async (container: HTMLDivElement) => {
    if (!selectedRoom) return;
    isFetchingOlderRef.current = true;
    const oldScrollHeight = container.scrollHeight;
    const oldScrollTop = container.scrollTop;

    await loadOlderMessages(selectedRoom);

    requestAnimationFrame(() => {
      const nextContainer = scrollContainerRef.current;
      if (nextContainer) {
        const heightDelta = nextContainer.scrollHeight - oldScrollHeight;
        nextContainer.scrollTop = oldScrollTop + heightDelta;
        lastScrollTopRef.current = nextContainer.scrollTop;
      }
      isFetchingOlderRef.current = false;
    });
  };

  if (!selectedRoom) {
    return (
      <div className="flex-1 flex flex-col items-center justify-center p-6 text-center text-muted-foreground select-none">
        <p className="text-sm">请从左侧列表选择一个聊天频道或房间开始浏览</p>
      </div>
    );
  }

  return (
    <div
      ref={scrollContainerRef}
      onScroll={handleScroll}
      className="flex-1 overflow-y-auto overflow-x-hidden p-2 sm:p-4 relative"
      style={{ scrollBehavior: "auto" }}
    >
      {/* Top Loading Indicator */}
      <div
        className={`transition-all duration-300 overflow-hidden flex justify-center items-center ${
          isLoadingMore ? "h-10 opacity-100 mb-2" : "h-0 opacity-0"
        }`}
      >
        <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full text-xs text-muted-foreground bg-muted/60 backdrop-blur-xs">
          <Loader2 className="w-3.5 h-3.5 animate-spin text-primary" />
          <span>正在加载更早历史记录...</span>
        </div>
      </div>

      {isLoadingMessages ? (
        <div className="flex h-full min-h-[300px] items-center justify-center">
          <div className="flex flex-col items-center gap-2 text-muted-foreground">
            <Loader2 className="w-6 h-6 animate-spin text-primary" />
            <span className="text-xs">加载会话消息中...</span>
          </div>
        </div>
      ) : (
        <div
          ref={messageContentRef}
          className="flex flex-col justify-end min-h-full max-w-4xl mx-auto w-full"
        >
          {/* History End Badge */}
          {!hasMoreHistory && messageGroups.length > 0 && (
            <div className="text-[11px] text-center text-muted-foreground/60 py-4 select-none">
              — 已到达本地数据库最早历史记录 —
            </div>
          )}

          {/* Empty State */}
          {messageGroups.length === 0 && (
            <div className="py-16 text-center text-sm text-muted-foreground">
              当前聊天室暂无历史消息
            </div>
          )}

          {/* Grouped Messages by Date */}
          {messageGroups.map((group) => (
            <div key={group.date} className="w-full mb-5">
              {/* Date Header: Centered on top of this date's messages */}
              <div className="w-full flex justify-center my-3 select-none">
                <DateBadge date={group.date} />
              </div>

              {/* Messages list in group */}
              <div className="flex flex-col gap-1.5 w-full">
                {group.messages.map((message) => (
                  <MessageBubble
                    key={message.id}
                    message={message}
                    room={selectedRoom}
                    userProfile={userProfile}
                  />
                ))}
              </div>
            </div>
          ))}

          {/* Bottom Newer Messages Button */}
          {hasNewerMessages && (
            <div className="flex justify-center py-4">
              <button
                type="button"
                onClick={() => void loadNewerMessages(selectedRoom)}
                disabled={isLoadingNewer}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-medium text-primary bg-primary/10 hover:bg-primary/20 transition-colors shadow-xs"
              >
                {isLoadingNewer ? (
                  <>
                    <Loader2 className="w-3.5 h-3.5 animate-spin" />
                    <span>加载中...</span>
                  </>
                ) : (
                  <span>加载后续较新消息 ↓</span>
                )}
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  );
};

export default ChatList;