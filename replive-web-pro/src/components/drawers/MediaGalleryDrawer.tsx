import {
  Film,
  Images,
  Loader2,
  Maximize2,
  MessageSquareShare,
  X,
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { cn, formatShortDate } from "../../lib/utils";
import useChatStore from "../../stores/chat-store";
import type { MediaItem, Message } from "../../types/chat";

const MEDIA_INITIAL_RENDER_COUNT = 18;
const MEDIA_RENDER_BATCH_SIZE = 12;
const MAX_CONCURRENT_IMAGE_LOADS = 4;
const LOAD_MORE_THRESHOLD = 160;

function VideoThumbnail({
  item,
  allowRemoteFallback,
}: {
  item: MediaItem;
  allowRemoteFallback: boolean;
}) {
  const [source, setSource] = useState(item.url);

  useEffect(() => {
    setSource(item.url);
  }, [item.url]);

  const handleError = () => {
    if (allowRemoteFallback && item.fallbackUrl && source !== item.fallbackUrl) {
      setSource(item.fallbackUrl);
    }
  };

  return (
    <video
      src={source}
      preload="metadata"
      muted
      playsInline
      className="w-full h-full object-cover opacity-80"
      onError={handleError}
    />
  );
}

export const MediaGalleryDrawer = () => {
  const mediaGalleryDrawerOpen = useChatStore((s) => s.mediaGalleryDrawerOpen);
  const setMediaGalleryDrawerOpen = useChatStore(
    (s) => s.setMediaGalleryDrawerOpen,
  );
  const mediaList = useChatStore((s) => s.mediaList);
  const isLoadingMedia = useChatStore((s) => s.isLoadingMedia);
  const openLightbox = useChatStore((s) => s.openLightbox);
  const jumpToMessage = useChatStore((s) => s.jumpToMessage);
  const selectedRoom = useChatStore((s) => s.selectedRoom);
  const userProfile = useChatStore((s) => s.userProfile);
  const allowRemoteFallback = userProfile?.offlineMode !== true;

  const [activeTab, setActiveTab] = useState<"all" | "image" | "video">("all");
  const [renderedMediaCount, setRenderedMediaCount] = useState(
    MEDIA_INITIAL_RENDER_COUNT,
  );
  const [loadingImageIds, setLoadingImageIds] = useState<Set<string>>(
    () => new Set(),
  );
  const [loadedImageIds, setLoadedImageIds] = useState<Set<string>>(
    () => new Set(),
  );
  const [failedImageIds, setFailedImageIds] = useState<Set<string>>(
    () => new Set(),
  );

  const [imageSources, setImageSources] = useState<Record<string, string>>({});
  const selectedRoomKey = selectedRoom
    ? (selectedRoom.category || "fandom") + ":" + selectedRoom.chatRoomId
    : "";

  const filteredMedia = useMemo(() => {
    if (activeTab === "all") return mediaList;
    return mediaList.filter((item) => item.type === activeTab);
  }, [mediaList, activeTab]);

  const renderedMedia = useMemo(
    () => filteredMedia.slice(0, renderedMediaCount),
    [filteredMedia, renderedMediaCount],
  );
  const renderedImageIds = useMemo(
    () =>
      renderedMedia
        .filter((item) => item.type === "image")
        .map((item) => item.id),
    [renderedMedia],
  );
  const hasMoreMedia = renderedMedia.length < filteredMedia.length;

  useEffect(() => {
    setRenderedMediaCount(MEDIA_INITIAL_RENDER_COUNT);
    setLoadingImageIds(new Set());
    setLoadedImageIds(new Set());
    setFailedImageIds(new Set());
    setImageSources({});
  }, [activeTab, mediaGalleryDrawerOpen, selectedRoomKey]);

  // 只让首屏少量原始图片并发下载，避免整屋媒体争抢同一 CDN 连接。
  useEffect(() => {
    if (!mediaGalleryDrawerOpen) return;

    const availableSlots = MAX_CONCURRENT_IMAGE_LOADS - loadingImageIds.size;
    if (availableSlots <= 0) return;

    const nextIds = renderedImageIds
      .filter(
        (id) =>
          !loadingImageIds.has(id) &&
          !loadedImageIds.has(id) &&
          !failedImageIds.has(id),
      )
      .slice(0, availableSlots);

    if (nextIds.length === 0) return;

    setLoadingImageIds((current) => {
      const next = new Set(current);
      for (const id of nextIds) {
        next.add(id);
      }
      return next;
    });
  }, [
    failedImageIds,
    loadedImageIds,
    loadingImageIds,
    mediaGalleryDrawerOpen,
    renderedImageIds,
  ]);
  const handleImageSettled = (id: string, succeeded: boolean) => {
    setLoadingImageIds((current) => {
      if (!current.has(id)) return current;
      const next = new Set(current);
      next.delete(id);
      return next;
    });

    const update = succeeded ? setLoadedImageIds : setFailedImageIds;
    update((current) => {
      if (current.has(id)) return current;
      const next = new Set(current);
      next.add(id);
      return next;
    });
  };

  const handleImageError = (item: MediaItem) => {
    const source = imageSources[item.id] || item.url;
    if (allowRemoteFallback && item.fallbackUrl && source !== item.fallbackUrl) {
      setImageSources((current) => ({ ...current, [item.id]: item.fallbackUrl! }));
      return;
    }
    handleImageSettled(item.id, false);
  };
  const handleGalleryScroll = (event: React.UIEvent<HTMLDivElement>) => {
    if (!hasMoreMedia) return;

    const target = event.currentTarget;
    if (
      target.scrollHeight - target.scrollTop - target.clientHeight >
      LOAD_MORE_THRESHOLD
    ) {
      return;
    }

    setRenderedMediaCount((current) =>
      Math.min(current + MEDIA_RENDER_BATCH_SIZE, filteredMedia.length),
    );
  };

  if (!mediaGalleryDrawerOpen) return null;

  const handleJumpToMessage = (item: MediaItem, e: React.MouseEvent) => {
    e.stopPropagation();
    if (!item.messageId) return;
    const dummyMsg: Message = {
      id: item.messageId,
      backendId: item.backendId,
      chatMessageId: item.messageId,
      content: "",
      type: item.type,
      createdAt: item.createdAt,
      mediaUrl: item.url,
      mediaFallbackUrl: item.fallbackUrl,
      senderId: "",
      senderName: item.senderName,
    };
    void jumpToMessage(dummyMsg);
    setMediaGalleryDrawerOpen(false);
  };

  return (
    <div className="fixed inset-0 z-50 overflow-hidden select-none animate-in fade-in duration-200">
      {/* Backdrop */}
      <div
        className="fixed inset-0 bg-black/50 backdrop-blur-xs transition-opacity"
        onClick={() => setMediaGalleryDrawerOpen(false)}
      />

      {/* Drawer Panel */}
      <aside className="fixed inset-y-0 right-0 w-full sm:w-[460px] bg-card border-l border-border shadow-2xl flex flex-col z-50 animate-in slide-in-from-right duration-300">
        {/* Header */}
        <div className="flex items-center justify-between px-4 py-3.5 border-b border-border/80 bg-card/90 backdrop-blur-md">
          <div className="flex items-center gap-2.5 min-w-0">
            <div className="p-2 rounded-xl bg-primary/15 text-primary">
              <Images className="w-4 h-4" />
            </div>
            <div className="flex items-center gap-2">
              <h2 className="text-sm font-bold text-foreground">相册</h2>
              <span className="px-2 py-0.5 text-xs rounded-full bg-muted font-mono text-muted-foreground">
                {mediaList.length}
              </span>
            </div>
          </div>

          <button
            type="button"
            onClick={() => setMediaGalleryDrawerOpen(false)}
            className="p-1.5 rounded-lg text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Tab Filters */}
        <div className="p-3 border-b border-border/60 bg-muted/20">
          <div className="grid grid-cols-3 gap-1 p-1 bg-muted/60 rounded-xl border border-border/40 text-xs">
            <button
              type="button"
              onClick={() => setActiveTab("all")}
              className={cn(
                "py-1.5 rounded-lg font-medium transition-all text-center",
                activeTab === "all"
                  ? "bg-card text-foreground font-semibold shadow-xs"
                  : "text-muted-foreground hover:text-foreground",
              )}
            >
              全部 ({mediaList.length})
            </button>
            <button
              type="button"
              onClick={() => setActiveTab("image")}
              className={cn(
                "py-1.5 rounded-lg font-medium transition-all text-center",
                activeTab === "image"
                  ? "bg-card text-foreground font-semibold shadow-xs"
                  : "text-muted-foreground hover:text-foreground",
              )}
            >
              图片 ({mediaList.filter((m) => m.type === "image").length})
            </button>
            <button
              type="button"
              onClick={() => setActiveTab("video")}
              className={cn(
                "py-1.5 rounded-lg font-medium transition-all text-center",
                activeTab === "video"
                  ? "bg-card text-foreground font-semibold shadow-xs"
                  : "text-muted-foreground hover:text-foreground",
              )}
            >
              视频 ({mediaList.filter((m) => m.type === "video").length})
            </button>
          </div>
        </div>

        {/* Gallery Grid */}
        <div
          className="flex-1 overflow-y-auto p-3"
          onScroll={handleGalleryScroll}
        >
          {isLoadingMedia && filteredMedia.length === 0 ? (
            <div className="py-20 flex flex-col items-center justify-center gap-2 text-muted-foreground text-xs">
              <Loader2 className="w-6 h-6 animate-spin text-primary" />
              <span>正在从数据库读取媒体列表...</span>
            </div>
          ) : filteredMedia.length === 0 ? (
            <div className="py-20 flex flex-col items-center justify-center text-center text-xs text-muted-foreground">
              <Images className="w-8 h-8 mb-2 opacity-40 stroke-1" />
              <span>该会话暂无相关媒体文件</span>
            </div>
          ) : (
            <div className="grid grid-cols-3 gap-2">
              {renderedMedia.map((item) => {
                const imageSource = imageSources[item.id] || item.url;
                const imageRequested =
                  loadingImageIds.has(item.id) ||
                  loadedImageIds.has(item.id) ||
                  failedImageIds.has(item.id);
                const imageLoaded = loadedImageIds.has(item.id);
                const imageFailed = failedImageIds.has(item.id);
                return (
                  <div
                    key={item.id}
                    onClick={() => openLightbox(item)}
                    className="group relative aspect-square rounded-xl overflow-hidden bg-muted/60 border border-border/50 cursor-pointer shadow-2xs hover:shadow-md transition-all"
                  >
                    {item.type === "image" ? (
                      <>
                        {imageRequested && (
                          <img
                            src={imageSource}
                            alt="媒体缩略图"
                            loading="eager"
                            fetchPriority="high"
                            className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-300"
                            decoding="sync"
                            onLoad={() => handleImageSettled(item.id, true)}
                            onError={() => handleImageError(item)}
                          />
                        )}
                        {!imageLoaded && !imageFailed && (
                          <span className="absolute inset-0 flex items-center justify-center pointer-events-none">
                            <Loader2 className="w-4 h-4 animate-spin text-muted-foreground/70" />
                          </span>
                        )}
                      </>
                    ) : (
                      <div className="w-full h-full relative bg-black flex items-center justify-center">
                        <VideoThumbnail
                          item={item}
                          allowRemoteFallback={allowRemoteFallback}
                        />
                        <span className="absolute inset-0 flex items-center justify-center bg-black/30">
                          <Film className="w-5 h-5 text-white/90" />
                        </span>
                      </div>
                    )}

                    {/* Hover Overlay with Action Buttons */}
                    <div className="absolute inset-0 bg-black/60 opacity-0 group-hover:opacity-100 transition-opacity flex flex-col justify-between p-2 text-white">
                      <div className="flex justify-between items-center text-[10px]">
                        <span className="font-mono bg-black/40 px-1.5 py-0.5 rounded backdrop-blur-xs">
                          {formatShortDate(item.createdAt)}
                        </span>
                        <span className="p-1 rounded bg-black/40 hover:bg-white/20 transition-colors">
                          <Maximize2 className="w-3 h-3" />
                        </span>
                      </div>

                      {item.messageId && (
                        <button
                          type="button"
                          onClick={(e) => handleJumpToMessage(item, e)}
                          className="w-full py-1 px-1.5 rounded-lg bg-primary/90 hover:bg-primary text-[10px] font-medium flex items-center justify-center gap-1 transition-colors shadow-xs"
                        >
                          <MessageSquareShare className="w-3 h-3" />
                          <span>定位消息</span>
                        </button>
                      )}
                    </div>
                  </div>
                );
              })}

              {hasMoreMedia && (
                <div className="col-span-3 flex h-10 items-center justify-center">
                  <Loader2 className="w-4 h-4 animate-spin text-muted-foreground/70" />
                </div>
              )}
            </div>
          )}
        </div>
      </aside>
    </div>
  );
};

export default MediaGalleryDrawer;


