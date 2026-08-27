import {
  CalendarDays,
  Images,
  Menu,
  RotateCw,
  Search,
} from "lucide-react";
import useChatStore from "../../stores/chat-store";
import useSettingsStore from "../../stores/settings-store";
import DayCountChip from "./DayCountChip";
import Avatar from "./Avatar";

export const ChatHeader = () => {
  const selectedRoom = useChatStore((s) => s.selectedRoom);
  const userProfile = useChatStore((s) => s.userProfile);
  const mediaList = useChatStore((s) => s.mediaList);
  const setSearchDrawerOpen = useChatStore((s) => s.setSearchDrawerOpen);
  const setMediaGalleryDrawerOpen = useChatStore(
    (s) => s.setMediaGalleryDrawerOpen,
  );
  const setDateJumpModalOpen = useChatStore((s) => s.setDateJumpModalOpen);
  const requestScrollToBottom = useChatStore((s) => s.requestScrollToBottom);

  const toggleSidebar = useSettingsStore((s) => s.toggleSidebar);

  return (
    <header className="flex items-center justify-between px-3 sm:px-5 py-2.5 border-b border-border/60 bg-background/85 backdrop-blur-md sticky top-0 z-30 select-none">
      {/* Left Info Area */}
      <div className="flex items-center gap-3 min-w-0">
        <button
          type="button"
          onClick={toggleSidebar}
          className="p-1.5 -ml-1 rounded-lg text-muted-foreground hover:text-foreground hover:bg-muted/80 transition-colors md:hidden"
          title="切换侧边栏"
        >
          <Menu className="w-5 h-5" />
        </button>

        {selectedRoom ? (
          <div className="flex items-center gap-2.5 min-w-0">
            <div className="relative shrink-0">
              <Avatar
                localUrl={selectedRoom.avatarLocalUrl}
                remoteUrl={userProfile?.offlineMode ? undefined : selectedRoom.avatarUrl}
                label={selectedRoom.displayName}
                className="w-9 h-9 rounded-full object-cover ring-1 ring-border/60 bg-muted"
                fallbackClassName="text-xs text-muted-foreground"
              />
            </div>
            <div className="flex flex-col min-w-0">
              <div className="flex items-center gap-2 min-w-0">
                <h1 className="text-sm font-bold text-foreground truncate max-w-[180px] sm:max-w-[280px] md:max-w-[360px]">
                  {selectedRoom.displayName}
                </h1>
                {selectedRoom.category !== "prime" &&
                  typeof selectedRoom.dayCount === "number" &&
                  selectedRoom.dayCount > 0 && (
                    <DayCountChip dayCount={selectedRoom.dayCount} />
                  )}
              </div>
              {selectedRoom.uniqueId && (
                <span className="text-[11px] text-muted-foreground font-mono truncate">
                  @{selectedRoom.uniqueId}
                </span>
              )}
            </div>
          </div>
        ) : (
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <span>未选择聊天对象</span>
          </div>
        )}
      </div>

      {/* Right Toolbar Action Buttons */}
      <div className="flex items-center gap-1 shrink-0">
        {/* Media Gallery Button (Title: 相册) */}
        <button
          type="button"
          onClick={() => setMediaGalleryDrawerOpen(true)}
          disabled={!selectedRoom}
          className="relative inline-flex items-center justify-center p-2 rounded-xl text-muted-foreground hover:text-foreground hover:bg-muted/80 disabled:opacity-40 transition-colors"
          title="相册"
        >
          <Images className="w-4 h-4" />
          {mediaList.length > 0 && (
            <span className="absolute top-1 right-1 w-2 h-2 rounded-full bg-primary ring-2 ring-background" />
          )}
        </button>

        {/* Search Drawer Button */}
        <button
          type="button"
          onClick={() => setSearchDrawerOpen(true)}
          disabled={!selectedRoom}
          className="inline-flex items-center justify-center p-2 rounded-xl text-muted-foreground hover:text-foreground hover:bg-muted/80 disabled:opacity-40 transition-colors"
          title="全文关键词检索 (Ctrl+F)"
        >
          <Search className="w-4 h-4" />
        </button>

        {/* Date Jump Modal Button */}
        <button
          type="button"
          onClick={() => setDateJumpModalOpen(true)}
          disabled={!selectedRoom}
          className="inline-flex items-center justify-center p-2 rounded-xl text-muted-foreground hover:text-foreground hover:bg-muted/80 disabled:opacity-40 transition-colors"
          title="日期跳转"
        >
          <CalendarDays className="w-4 h-4" />
        </button>

        {/* Scroll To Bottom Button */}
        <button
          type="button"
          onClick={() => void requestScrollToBottom()}
          disabled={!selectedRoom}
          className="inline-flex items-center justify-center p-2 rounded-xl text-muted-foreground hover:text-foreground hover:bg-muted/80 disabled:opacity-40 transition-colors"
          title="拉取最新并滑到底部"
        >
          <RotateCw className="w-4 h-4" />
        </button>
      </div>
    </header>
  );
};

export default ChatHeader;
