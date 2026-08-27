import { ExternalLink, Play } from "lucide-react";
import { memo, useEffect, useState } from "react";
import { cn, formatTimeStr } from "../../lib/utils";
import useChatStore from "../../stores/chat-store";
import type { ChatRoom, Message, UserProfile } from "../../types/chat";
import Avatar from "./Avatar";

interface MessageBubbleProps {
  message: Message;
  room: ChatRoom;
  userProfile: UserProfile | null;
  isHighlighted?: boolean;
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
  ({ message, room, userProfile, isHighlighted }: MessageBubbleProps) => {
    const openLightbox = useChatStore((s) => s.openLightbox);
    const { mediaSrc, fallbackToRemote } = useFallbackMediaSource(
      message.mediaUrl,
      userProfile?.offlineMode ? undefined : message.mediaFallbackUrl,
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

    const avatarLocalUrl = isMine ? userProfile?.avatarLocalUrl : room.avatarLocalUrl;
    const avatarRemoteUrl = userProfile?.offlineMode
      ? undefined
      : isMine
        ? userProfile?.avatarUrl
        : room.avatarUrl;

    return (
      <div
        id={`msg-${message.id}`}
        data-backend-id={message.backendId > 0 ? message.backendId : undefined}
        className={cn(
          "group flex items-start gap-3 py-1.5 px-2 transition-all duration-300 rounded-xl",
          isMine ? "flex-row-reverse" : "flex-row",
          isHighlighted && "msg-row-highlight",
        )}
      >
        {/* Avatar */}
        <div className="relative shrink-0 select-none">
          <Avatar
            localUrl={avatarLocalUrl}
            remoteUrl={avatarRemoteUrl}
            label={isMine ? userProfile?.displayName || "我" : message.senderName || room.displayName}
            loading="lazy"
            decoding="async"
            className="w-9 h-9 rounded-full object-cover bg-muted ring-1 ring-border/50 shadow-xs"
            fallbackClassName="text-xs text-muted-foreground"
          />
        </div>

        {/* Message Container */}
        <div
          className={cn(
            "flex flex-col max-w-[85%] sm:max-w-[70%] lg:max-w-[55%] min-w-0",
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
              <span className="text-[10px] text-primary/80 font-mono">
                既読
              </span>
            )}
          </div>

          {/* Bubble + Reaction Container */}
          <div className="relative max-w-full flex flex-col">
            {/* Bubble Box */}
            <div
              className={cn(
                "relative rounded-2xl shadow-xs transition-shadow overflow-hidden",
              isMine
                ? "bg-primary text-primary-foreground"
                : "bg-card text-card-foreground border border-border/50",
            )}
          >
            {/* Coin Amount (Prime Chat) */}
            {typeof message.coinAmount === "number" &&
              message.coinAmount > 0 && (
                <div className="px-3.5 pt-2 pb-0 text-[11px] font-semibold text-amber-300 flex items-center gap-1">
                  <span>🪙 {message.coinAmount} coins</span>
                </div>
              )}

            {/* Text Message */}
            {message.type === "text" && (
              <div className="px-3.5 py-2.5">
                <p
                  className={cn(
                    "text-[13.5px] leading-relaxed whitespace-pre-wrap break-words select-text",
                    message.isDeleted && "italic text-muted-foreground text-xs",
                  )}
                >
                  {message.content}
                </p>
              </div>
            )}

            {/* Image / Video Media */}
            {(message.type === "image" || message.type === "video") &&
              mediaSrc && (
                <div className="relative group/media overflow-hidden">
                  {message.type === "image" ? (
                    <div
                      role="button"
                      tabIndex={0}
                      onClick={handleMediaClick}
                      onKeyDown={(e) => e.key === "Enter" && handleMediaClick()}
                      className="cursor-pointer overflow-hidden min-w-[180px] max-w-[280px] sm:max-w-[340px] max-h-[300px] min-h-[120px] bg-muted/40 flex items-center justify-center"
                    >
                      <img
                        src={mediaSrc}
                        alt="聊天图片"
                        className="w-full h-full object-cover transition-transform duration-300 hover:scale-[1.03]"
                        decoding="async"
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
                      className="cursor-pointer overflow-hidden w-[280px] sm:w-[340px] aspect-video max-h-[300px] bg-black/80 flex items-center justify-center relative group/video"
                    >
                      <video
                        src={mediaSrc}
                        preload="metadata"
                        muted
                        playsInline
                        className="w-full h-full object-cover"
                        onError={fallbackToRemote}
                      />
                      <div className="absolute inset-0 bg-black/30 group-hover/video:bg-black/40 transition-colors flex items-center justify-center">
                        <span className="p-3 rounded-full bg-primary/90 text-primary-foreground shadow-lg backdrop-blur-xs group-hover/video:scale-110 transition-transform">
                          <Play className="w-5 h-5 fill-current" />
                        </span>
                      </div>
                    </div>
                  )}
                </div>
              )}
          </div>

            {/* Reaction Emoji (Prime Chat) */}
            {message.reactionEmoji && (
              <div className="pt-1 text-[15px] select-none pl-3.5 flex justify-start">
                <span>{message.reactionEmoji}</span>
              </div>
            )}
          </div>
        </div>
      </div>
    );
  },
);

export default MessageBubble;
