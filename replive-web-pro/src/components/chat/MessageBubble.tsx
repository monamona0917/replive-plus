import { ExternalLink, Play } from "lucide-react";
import { memo, useEffect, useState } from "react";
import { cn, formatTimeStr } from "../../lib/utils";
import useChatStore from "../../stores/chat-store";
import type { ChatRoom, Message, UserProfile } from "../../types/chat";

interface MessageBubbleProps {
  message: Message;
  room: ChatRoom;
  userProfile: UserProfile | null;
}

function useFallbackMediaSource(primary?: string, fallback?: string) {
  const [mediaSrc, setMediaSrc] = useState(primary);

  useEffect(() => {
    setMediaSrc(primary);
  }, [primary]);

  const fallbackToRemote = () => {
    if (fallback && mediaSrc !== fallback) {
      setMediaSrc(fallback);
    }
  };

  return { mediaSrc, fallbackToRemote };
}

export const MessageBubble = memo(
  ({ message, room, userProfile }: MessageBubbleProps) => {
    const openLightbox = useChatStore((s) => s.openLightbox);
    const isJumpTarget = useChatStore(
      (s) => s.jumpTargetMessageId === message.id,
    );
    const { mediaSrc, fallbackToRemote } = useFallbackMediaSource(
      message.mediaUrl,
      message.mediaFallbackUrl,
    );

    // Fandom 的本地 user_id 是房间归属，发送者需由昵称判断；Prime 有结构化发送者字段。
    const isMine =
      room.category === "fandom"
        ? Boolean(message.senderName) && message.senderName !== room.displayName
        : message.senderKind === "member" ||
          message.senderId === userProfile?.userId;

    const isReadByTalent =
      room.category === "fandom" &&
      isMine &&
      typeof room.talentLastCheckTime === "number" &&
      Number.isFinite(room.talentLastCheckTime) &&
      new Date(message.createdAt).getTime() <= room.talentLastCheckTime;

    const senderLabel = isMine
      ? userProfile?.displayName || message.senderName || "me"
      : message.senderName || room.displayName;

    const reactionLabel =
      message.senderKind === "member"
        ? "对方添加的 reaction"
        : message.senderKind === "talent"
          ? "我添加的 reaction"
          : "reaction";

    const handleMediaClick = () => {
      if (!mediaSrc) return;
      openLightbox({
        id: `media-${message.id}`,
        type: message.type === "video" ? "video" : "image",
        url: mediaSrc,
        fallbackUrl:
          message.mediaFallbackUrl && message.mediaFallbackUrl !== mediaSrc
            ? message.mediaFallbackUrl
            : undefined,
        createdAt: message.createdAt,
        senderName: message.senderName,
        messageId: message.id,
        backendId: message.backendId,
      });
    };

    const avatarUrl = isMine
      ? userProfile?.avatarUrl ||
        "https://api.dicebear.com/7.x/bottts/svg?seed=user_me"
      : room.avatarUrl ||
        `https://api.dicebear.com/7.x/identicon/svg?seed=${encodeURIComponent(message.senderName || room.displayName)}`;

    return (
      <div
        id={`msg-${message.id}`}
        className={cn(
          "group flex items-start gap-3 py-1.5 px-2 transition-all duration-300 rounded-xl",
          isMine ? "flex-row-reverse" : "flex-row",
          isJumpTarget && "msg-jump-highlight",
        )}
      >
        {/* Avatar */}
        <div className="relative shrink-0 select-none">
          <img
            src={avatarUrl}
            alt={isMine ? "我" : message.senderName || room.displayName}
            loading="lazy"
            decoding="async"
            className="w-9 h-9 rounded-full object-cover bg-muted ring-1 ring-border/50 shadow-xs"
            onError={(e) => {
              (e.currentTarget as HTMLImageElement).src =
                "https://api.dicebear.com/7.x/shapes/svg?seed=avatar_fallback";
            }}
          />
        </div>

        {/* Message Container */}
        <div
          className={cn(
            "flex flex-col max-w-[82%] sm:max-w-[32rem] min-w-0",
            isMine ? "items-end" : "items-start",
          )}
        >
          {/* Header Info */}
          <div
            className={cn(
              "flex items-center gap-2 mb-1 px-1 text-xs select-none",
              isMine ? "flex-row-reverse" : "flex-row",
            )}
          >
            <span className="font-semibold text-foreground/90 truncate max-w-[150px]">
              {senderLabel}
            </span>
            <span className="text-[11px] text-muted-foreground font-mono">
              {formatTimeStr(message.createdAt)}
            </span>
            {isReadByTalent && (
              <span className="text-[11px] text-muted-foreground font-medium">
                {"\u65e2\u8aad"}
              </span>
            )}
          </div>

          {/* Bubble Box */}
          <div
            className={cn(
              "relative rounded-2xl shadow-xs transition-shadow overflow-hidden",
              isMine
                ? "bg-primary text-primary-foreground rounded-tr-xs"
                : "bg-card text-card-foreground border border-border/50 rounded-tl-xs",
            )}
          >
            {/* Text Message */}
            {message.type === "text" && (
              <div className="px-3.5 py-2.5">
                <p className="text-[13.5px] leading-relaxed whitespace-pre-wrap break-words select-text">
                  {message.content}
                </p>
              </div>
            )}

            {/* Image / Video Media */}
            {(message.type === "image" || message.type === "video") &&
              mediaSrc && (
                <div className="relative group/media overflow-hidden rounded-xl">
                  {message.type === "image" ? (
                    <div
                      role="button"
                      tabIndex={0}
                      onClick={handleMediaClick}
                      onKeyDown={(e) => e.key === "Enter" && handleMediaClick()}
                      className="cursor-pointer overflow-hidden w-[20rem] max-w-full max-h-[18rem] bg-muted/40 flex items-center justify-center"
                    >
                      <img
                        src={mediaSrc}
                        alt="聊天图片"
                        loading="eager"
                        fetchPriority="high"
                        className="w-full h-auto max-h-[18rem] object-cover transition-transform duration-300 hover:scale-[1.03]"
                        decoding="sync"
                        onError={fallbackToRemote}
                      />
                      <div className="absolute inset-0 bg-black/0 group-hover/media:bg-black/15 transition-colors flex items-center justify-center opacity-0 group-hover/media:opacity-100">
                        <span className="p-1.5 rounded-full bg-black/50 text-white backdrop-blur-xs">
                          <ExternalLink className="w-4 h-4" />
                        </span>
                      </div>
                    </div>
                  ) : (
                    <div
                      role="button"
                      tabIndex={0}
                      onClick={handleMediaClick}
                      onKeyDown={(e) => e.key === "Enter" && handleMediaClick()}
                      className="relative cursor-pointer w-[20rem] max-w-full aspect-video bg-black rounded-xl overflow-hidden flex items-center justify-center"
                    >
                      <video
                        key={mediaSrc}
                        src={mediaSrc}
                        preload="metadata"
                        muted
                        className="w-full h-full object-cover opacity-90"
                        onError={fallbackToRemote}
                      />
                      <div className="absolute inset-0 flex items-center justify-center bg-black/25 group-hover/media:bg-black/40 transition-colors">
                        <span className="p-3 rounded-full bg-white/25 text-white backdrop-blur-md shadow-lg group-hover/media:scale-110 transition-transform">
                          <Play className="w-5 h-5 fill-white ml-0.5" />
                        </span>
                      </div>
                    </div>
                  )}

                  {/* Caption text if any */}
                  {message.content &&
                    !message.content.startsWith("[") &&
                    message.content.length > 0 && (
                      <p className="px-3 py-2 text-xs text-muted-foreground bg-background/80 backdrop-blur-xs">
                        {message.content}
                      </p>
                    )}
                </div>
              )}
          </div>

          {message.reactionEmoji && (
            <div
              className={cn(
                "mt-1 inline-flex items-center gap-1 rounded-full border border-border/70 bg-background px-2 py-0.5 text-sm leading-none shadow-xs",
                isMine ? "self-end" : "self-start",
              )}
              title={reactionLabel}
              aria-label={reactionLabel}
            >
              <span aria-hidden="true">{message.reactionEmoji}</span>
            </div>
          )}
        </div>
      </div>
    );
  },
);

MessageBubble.displayName = "MessageBubble";

export default MessageBubble;
