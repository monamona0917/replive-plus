import {
  ChevronLeft,
  ChevronRight,
  Download,
  RotateCw,
  X,
  ZoomIn,
  ZoomOut,
} from "lucide-react";
import { useEffect, useState } from "react";
import { formatTimeStr } from "../../lib/utils";
import useChatStore from "../../stores/chat-store";

export const MediaLightbox = () => {
  const lightboxMedia = useChatStore((s) => s.lightboxMedia);
  const mediaList = useChatStore((s) => s.mediaList);
  const closeLightbox = useChatStore((s) => s.closeLightbox);
  const stepLightbox = useChatStore((s) => s.stepLightbox);

  const [scale, setScale] = useState(1);
  const [rotation, setRotation] = useState(0);
  const [mediaSrc, setMediaSrc] = useState("");
  const [mediaSourceId, setMediaSourceId] = useState("");

  // 重置变换参数和本地优先媒体地址。
  useEffect(() => {
    setScale(1);
    setRotation(0);
    setMediaSrc(lightboxMedia?.url || "");
    setMediaSourceId(lightboxMedia?.id || "");
  }, [lightboxMedia]);
  // 键盘快捷键监听
  useEffect(() => {
    if (!lightboxMedia) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") closeLightbox();
      else if (e.key === "ArrowLeft") stepLightbox(-1);
      else if (e.key === "ArrowRight") stepLightbox(1);
      else if (e.key === "+" || e.key === "=")
        setScale((prev) => Math.min(prev + 0.25, 3));
      else if (e.key === "-") setScale((prev) => Math.max(prev - 0.25, 0.5));
      else if (e.key === "r" || e.key === "R")
        setRotation((prev) => (prev + 90) % 360);
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [lightboxMedia, closeLightbox, stepLightbox]);

  if (!lightboxMedia) return null;

  const currentIndex = mediaList.findIndex((m) => m.id === lightboxMedia.id);
  const totalCount = mediaList.length;
  const activeMediaSrc =
    mediaSourceId === lightboxMedia.id ? mediaSrc : lightboxMedia.url;

  const fallbackToRemote = () => {
    if (
      lightboxMedia.fallbackUrl &&
      activeMediaSrc !== lightboxMedia.fallbackUrl
    ) {
      setMediaSrc(lightboxMedia.fallbackUrl);
    }
  };
  const handleDownload = () => {
    const a = document.createElement("a");
    a.href = activeMediaSrc;
    a.download = `replive_${lightboxMedia.id}_${Date.now()}`;
    a.target = "_blank";
    a.rel = "noreferrer";
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  };

  return (
    <div className="fixed inset-0 z-50 flex flex-col bg-black/92 backdrop-blur-xl text-white select-none transition-all">
      {/* Top Header Bar */}
      <div className="flex items-center justify-between px-4 sm:px-6 py-3.5 bg-black/40 border-b border-white/10 z-10">
        <div className="flex items-center gap-3 min-w-0">
          <span className="text-xs font-semibold px-2 py-0.5 rounded-full bg-white/15 text-white/90">
            {currentIndex !== -1
              ? `${currentIndex + 1} / ${totalCount}`
              : "媒体预览"}
          </span>
          <div className="flex flex-col min-w-0">
            <span className="text-xs font-medium text-white/90 truncate">
              {lightboxMedia.senderName}
            </span>
            <span className="text-[10px] text-white/50 font-mono">
              {formatTimeStr(lightboxMedia.createdAt)}
            </span>
          </div>
        </div>

        {/* Toolbar controls */}
        <div className="flex items-center gap-1.5">
          {lightboxMedia.type === "image" && (
            <>
              <button
                type="button"
                onClick={() => setScale((prev) => Math.max(prev - 0.25, 0.5))}
                className="p-2 rounded-xl text-white/70 hover:text-white hover:bg-white/10 transition-colors"
                title="缩小 (-)"
              >
                <ZoomOut className="w-4 h-4" />
              </button>
              <button
                type="button"
                onClick={() => setScale((prev) => Math.min(prev + 0.25, 3))}
                className="p-2 rounded-xl text-white/70 hover:text-white hover:bg-white/10 transition-colors"
                title="放大 (+)"
              >
                <ZoomIn className="w-4 h-4" />
              </button>
              <button
                type="button"
                onClick={() => setRotation((prev) => (prev + 90) % 360)}
                className="p-2 rounded-xl text-white/70 hover:text-white hover:bg-white/10 transition-colors"
                title="顺时针旋转 (R)"
              >
                <RotateCw className="w-4 h-4" />
              </button>
            </>
          )}

          <button
            type="button"
            onClick={handleDownload}
            className="p-2 rounded-xl text-white/70 hover:text-white hover:bg-white/10 transition-colors"
            title="下载原文件"
          >
            <Download className="w-4 h-4" />
          </button>

          <div className="w-[1px] h-4 bg-white/20 mx-1" />

          <button
            type="button"
            onClick={closeLightbox}
            className="p-2 rounded-xl text-white/70 hover:text-white hover:bg-white/15 transition-colors"
            title="关闭 (Esc)"
          >
            <X className="w-5 h-5" />
          </button>
        </div>
      </div>

      {/* Main Viewport Content */}
      <div
        className="flex-1 relative flex items-center justify-center p-4 sm:p-8 overflow-hidden"
        onClick={(e) => {
          if (e.target === e.currentTarget) closeLightbox();
        }}
      >
        {/* Previous Button */}
        {totalCount > 1 && (
          <button
            type="button"
            onClick={() => stepLightbox(-1)}
            className="absolute left-4 top-1/2 -translate-y-1/2 p-3 rounded-full bg-black/40 text-white/80 hover:text-white hover:bg-black/80 backdrop-blur-md border border-white/10 transition-all z-20"
            title="上一张 (←)"
          >
            <ChevronLeft className="w-6 h-6" />
          </button>
        )}

        {/* Media render */}
        <div className="relative max-w-full max-h-full flex items-center justify-center transition-transform duration-200">
          {lightboxMedia.type === "image" ? (
            <img
              src={activeMediaSrc}
              alt="全屏图片"
              onError={fallbackToRemote}
              style={{
                transform: `scale(${scale}) rotate(${rotation}deg)`,
                transition: "transform 0.2s ease-out",
              }}
              className="max-w-[90vw] max-h-[82vh] object-contain rounded-lg shadow-2xl"
            />
          ) : (
            <video
              src={activeMediaSrc}
              controls
              autoPlay
              className="max-w-[90vw] max-h-[82vh] object-contain rounded-lg shadow-2xl bg-black"
            />
          )}
        </div>

        {/* Next Button */}
        {totalCount > 1 && (
          <button
            type="button"
            onClick={() => stepLightbox(1)}
            className="absolute right-4 top-1/2 -translate-y-1/2 p-3 rounded-full bg-black/40 text-white/80 hover:text-white hover:bg-black/80 backdrop-blur-md border border-white/10 transition-all z-20"
            title="下一张 (→)"
          >
            <ChevronRight className="w-6 h-6" />
          </button>
        )}
      </div>
    </div>
  );
};

export default MediaLightbox;

