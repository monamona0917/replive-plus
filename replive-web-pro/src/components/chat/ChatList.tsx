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
  const [highlightedMessageId, setHighlightedMessageId] = useState<string | null>(null);

  const selectedRoom = useChatStore((s) => s.selectedRoom);
  const userProfile = useChatStore((s) => s.userProfile);
  const messagesByRoom = useChatStore((s) => s.messagesByRoom);
  const hasMoreByRoom = useChatStore((s) => s.hasMoreByRoom);
  const hasNewerByRoom = useChatStore((s) => s.hasNewerByRoom);
  const isLoadingMessagesByRoom = useChatStore((s) => s.isLoadingMessagesByRoom);
  const isLoadingMore = useChatStore((s) => s.isLoadingMore);
  const jumpTarget = useChatStore((s) => s.jumpTarget);
  const scrollToBottomToken = useChatStore((s) => s.scrollToBottomToken);

  const loadOlderMessages = useChatStore((s) => s.loadOlderMessages);
  const clearJumpTarget = useChatStore((s) => s.clearJumpTarget);
  const setError = useChatStore((s) => s.setError);

  const activeKey = selectedRoom ? roomKey(selectedRoom) : "";
  const isJumping = Boolean(jumpTarget && jumpTarget.roomKey === activeKey);

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

  // 切换房间时先贴住底部，直到用户主动向上浏览历史消息（跳转中则不贴底）。
  useLayoutEffect(() => {
    const shouldKeepAtBottom = Boolean(activeKey);
    keepAtBottomRef.current = shouldKeepAtBottom;
    lastScrollTopRef.current = 0;
    setKeepAtBottomOnRoomEntry(shouldKeepAtBottom);
  }, [activeKey]);

  // 跳转位置优先于首次进入时的贴底逻辑。
  useLayoutEffect(() => {
    if (!isJumping) return;
    keepAtBottomRef.current = false;
    setKeepAtBottomOnRoomEntry(false);
  }, [isJumping]);

  // 首次进入房间后定位到最新消息。
  useEffect(() => {
    const container = scrollContainerRef.current;
    if (
      !container ||
      !activeKey ||
      !keepAtBottomRef.current ||
      isJumping ||
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
    isJumping,
    keepAtBottomOnRoomEntry,
  ]);

  // 图片等媒体的固有尺寸在加载后会改变消息内容高度，继续维持首次进入时的底部锚点（跳转时不执行）。
  useEffect(() => {
    const content = messageContentRef.current;
    if (
      !content ||
      !activeKey ||
      !keepAtBottomOnRoomEntry ||
      isJumping ||
      isLoadingMessages
    ) {
      return;
    }

    let animationFrameId = 0;
    const keepAtBottom = () => {
      if (
        !keepAtBottomRef.current ||
        isJumping ||
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
          isJumping ||
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
    isJumping,
    keepAtBottomOnRoomEntry,
  ]);

  // 确定性瞬时居中定位 + 尺寸稳定监听 + 1.5s 高亮与锁释放
  useEffect(() => {
    if (
      !jumpTarget ||
      jumpTarget.roomKey !== activeKey ||
      !jumpTarget.messageId ||
      isLoadingMessages
    ) {
      return;
    }

    const targetMessageId = jumpTarget.messageId;
    const currentRequestId = jumpTarget.requestId;
    let cancelled = false;
    let frameCount = 0;
    let locateFrame = 0;
    let releaseTimer: number | null = null;
    let timeoutId: number | null = null;
    let stabilizationObserver: ResizeObserver | null = null;
    let mutationObserver: MutationObserver | null = null;
    let located = false;
    const maxFrames = 180; // 给图片较多的历史窗口最多约 3 秒完成挂载

    const findTargetElement = () => {
      const backendId = jumpTarget.backendId;
      if (backendId && backendId > 0) {
        const byBackendId = document.querySelector<HTMLElement>(
          `[data-backend-id="${backendId}"]`,
        );
        if (byBackendId) return byBackendId;
      }
      const byMessageId = document.getElementById(`msg-${targetMessageId}`);
      if (byMessageId) return byMessageId;

      // 日期跳转的日期分组是稳定的 DOM 锚点，即使目标图片消息尚未完成挂载，
      // 也可以先把视图定位到正确日期，避免误判为定位失败后停在窗口顶部。
      if (jumpTarget.date) {
        return document.querySelector<HTMLElement>(
          `[data-date-key="${jumpTarget.date}"]`,
        );
      }
      return null;
    };

    const attemptLocate = () => {
      if (cancelled || located) return;
      const el = findTargetElement();
      const container = scrollContainerRef.current;

      if (el && container) {
        located = true;
        // 使用视口相对位置，避免消息嵌套在日期分组时 offsetTop 参照物不同。
        const containerRect = container.getBoundingClientRect();
        const targetRect = el.getBoundingClientRect();
        const targetCenterOffset =
          targetRect.top - containerRect.top -
          (container.clientHeight - targetRect.height) / 2;
        container.scrollTop = Math.max(0, container.scrollTop + targetCenterOffset);
        lastScrollTopRef.current = container.scrollTop;

        // 设置高亮
        setHighlightedMessageId(targetMessageId);

        // 启动 ResizeObserver 在内容尺寸微调（如图片解码撑开）时重新校准居中
        const content = messageContentRef.current;
        if (content) {
          stabilizationObserver = new ResizeObserver(() => {
            if (cancelled) return;
            const latestEl = findTargetElement();
            if (latestEl && container) {
              const latestContainerRect = container.getBoundingClientRect();
              const latestRect = latestEl.getBoundingClientRect();
              const latestCenterOffset =
                latestRect.top - latestContainerRect.top -
                (container.clientHeight - latestRect.height) / 2;
              container.scrollTop = Math.max(
                0,
                container.scrollTop + latestCenterOffset,
              );
              lastScrollTopRef.current = container.scrollTop;
            }
          });
          stabilizationObserver.observe(content);
        }

        // 1.5 秒后安全解除高亮与跳转锁
        releaseTimer = window.setTimeout(() => {
          if (!cancelled) {
            stabilizationObserver?.disconnect();
            stabilizationObserver = null;
            setHighlightedMessageId(null);
            clearJumpTarget(currentRequestId);
          }
        }, 1500);
        return;
      }

      frameCount++;
      if (frameCount < maxFrames) {
        locateFrame = requestAnimationFrame(attemptLocate);
      } else {
        // 交给下面的超时兜底；MutationObserver 仍可在此期间发现目标节点。
      }
    };

    // React 渲染和图片布局可能跨越多个帧，监听节点变化可在目标出现时立即重试。
    mutationObserver = new MutationObserver(() => attemptLocate());
    if (messageContentRef.current) {
      mutationObserver.observe(messageContentRef.current, {
        childList: true,
        subtree: true,
      });
    }
    locateFrame = requestAnimationFrame(attemptLocate);
    timeoutId = window.setTimeout(() => {
      if (!located && !cancelled) {
        setError("未能定位到目标消息位置");
        clearJumpTarget(currentRequestId);
      }
    }, 5000);

    return () => {
      cancelled = true;
      if (locateFrame) cancelAnimationFrame(locateFrame);
      if (releaseTimer !== null) window.clearTimeout(releaseTimer);
      if (timeoutId !== null) window.clearTimeout(timeoutId);
      mutationObserver?.disconnect();
      mutationObserver = null;
      stabilizationObserver?.disconnect();
      stabilizationObserver = null;
    };
  }, [jumpTarget, activeKey, clearJumpTarget, setError, isLoadingMessages]);

  // 手动请求滚动到底部
  useEffect(() => {
    const container = scrollContainerRef.current;
    if (!container || scrollToBottomToken === 0) return;

    requestAnimationFrame(() => {
      container.scrollTo({ top: container.scrollHeight, behavior: "smooth" });
    });
  }, [scrollToBottomToken]);

  // 滚动监听与向上防跳跃加载（跳转中或 loading 期间彻底屏蔽）
  const handleScroll = (e: React.UIEvent<HTMLDivElement>) => {
    if (isLoadingMessages || isJumping) {
      return;
    }

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

  };

  const triggerLoadMore = async (container: HTMLDivElement) => {
    if (!selectedRoom || isJumping) return;
    isFetchingOlderRef.current = true;
    const oldScrollHeight = container.scrollHeight;
    const oldScrollTop = container.scrollTop;

    await loadOlderMessages(selectedRoom);

    requestAnimationFrame(() => {
      // 如果等待期间发起了跳转，跳过高度补偿与位置变动
      const currentJumpTarget = useChatStore.getState().jumpTarget;
      if (currentJumpTarget && currentJumpTarget.roomKey === activeKey) {
        isFetchingOlderRef.current = false;
        return;
      }

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
        <p className="text-sm">请从左侧列表选择一个聊天对象</p>
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

      {isLoadingMessages && messageGroups.length === 0 ? (
        <div className="flex h-full min-h-[300px] items-center justify-center">
          <div className="flex flex-col items-center gap-2 text-muted-foreground">
            <Loader2 className="w-6 h-6 animate-spin text-primary" />
            <span className="text-xs">加载会话消息中...</span>
          </div>
        </div>
      ) : (
        <>
          {isLoadingMessages && (
            <div className="sticky top-2 z-10 flex justify-center pointer-events-none">
              <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full text-xs text-muted-foreground bg-background/85 border border-border/60 shadow-sm backdrop-blur-sm">
                <Loader2 className="w-3.5 h-3.5 animate-spin text-primary" />
                <span>正在定位消息...</span>
              </div>
            </div>
          )}
          <div
            ref={messageContentRef}
            className="flex flex-col justify-end min-h-full w-full max-w-none px-2 sm:px-6 lg:px-8"
          >
            {/* History End Badge */}
            {!hasMoreHistory && messageGroups.length > 0 && (
              <div className="text-[11px] text-center text-muted-foreground/60 py-4 select-none">
                -已无更早聊天记录-
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
              <div
                key={group.date}
                data-date-key={group.date}
                className="w-full mb-5"
              >
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
                      isHighlighted={highlightedMessageId === message.id}
                    />
                  ))}
                </div>
              </div>
            ))}

          </div>
        </>
      )}
    </div>
  );
};

export default ChatList;
