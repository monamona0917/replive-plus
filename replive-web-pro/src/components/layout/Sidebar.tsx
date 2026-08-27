import {
  ChevronLeft,
  Info,
  Moon,
  Sun,
} from "lucide-react";
import { useMemo } from "react";
import { cn, formatShortDate } from "../../lib/utils";
import useChatStore, { roomKey } from "../../stores/chat-store";
import useSettingsStore from "../../stores/settings-store";
import type { ChatRoom } from "../../types/chat";
import Avatar from "../chat/Avatar";

export const Sidebar = () => {
  const activeCategory = useChatStore((s) => s.activeCategory);
  const setActiveCategory = useChatStore((s) => s.setActiveCategory);
  const rooms = useChatStore((s) => s.rooms);
  const selectedRoom = useChatStore((s) => s.selectedRoom);
  const selectRoom = useChatStore((s) => s.selectRoom);
  const messagesByRoom = useChatStore((s) => s.messagesByRoom);
  const userProfile = useChatStore((s) => s.userProfile);
  const setAboutModalOpen = useChatStore((s) => s.setAboutModalOpen);

  const sidebarCollapsed = useSettingsStore((s) => s.sidebarCollapsed);
  const toggleSidebar = useSettingsStore((s) => s.toggleSidebar);
  const theme = useSettingsStore((s) => s.theme);
  const setTheme = useSettingsStore((s) => s.setTheme);

  // 分类统计
  const fandomRooms = useMemo(
    () => rooms.filter((r) => r.category === "fandom"),
    [rooms],
  );
  const primeRooms = useMemo(
    () => rooms.filter((r) => r.category === "prime"),
    [rooms],
  );

  const displayedRooms = useMemo(() => {
    const roomsForCategory = activeCategory === "fandom" ? fandomRooms : primeRooms;
    const lastActivityTime = (room: ChatRoom) => {
      const messages = messagesByRoom[roomKey(room)];
      const latestMessage = messages?.[messages.length - 1];
      const timestamp = latestMessage?.createdAt ?? room.lastMessageTime;
      const parsed = timestamp ? Date.parse(timestamp) : 0;
      return Number.isFinite(parsed) ? parsed : 0;
    };

    return [...roomsForCategory].sort((left, right) => {
      const timeDifference = lastActivityTime(right) - lastActivityTime(left);
      if (timeDifference !== 0) return timeDifference;
      return left.displayName.localeCompare(right.displayName, "ja");
    });
  }, [activeCategory, fandomRooms, primeRooms, messagesByRoom]);

  const handleRoomClick = (room: ChatRoom) => {
    void selectRoom(room);
    if (window.innerWidth < 768) {
      toggleSidebar();
    }
  };

  const handleThemeToggle = () => {
    setTheme(theme === "dark" ? "light" : "dark");
  };

  return (
    <aside
      className={cn(
        "flex flex-col h-full bg-sidebar border-r border-sidebar-border select-none transition-all duration-300 z-50",
        // Desktop
        sidebarCollapsed ? "md:w-[72px]" : "md:w-80",
        // Mobile drawer overlay
        "fixed inset-y-0 left-0 w-80 md:static md:translate-x-0 shadow-xl md:shadow-none",
        sidebarCollapsed ? "-translate-x-full md:translate-x-0" : "translate-x-0",
      )}
    >
      {/* Sidebar Header */}
      <div
        className={cn(
          "flex items-center py-3.5 border-b border-sidebar-border transition-all",
          sidebarCollapsed ? "justify-center px-2" : "justify-between px-4",
        )}
      >
        {!sidebarCollapsed && (
          <div className="flex items-center min-w-0">
            <span className="font-bold text-base text-sidebar-foreground tracking-tight">
              Replive+
            </span>
          </div>
        )}

        <button
          type="button"
          onClick={toggleSidebar}
          className="p-1.5 rounded-lg text-muted-foreground hover:text-foreground hover:bg-sidebar-accent transition-colors"
          title={sidebarCollapsed ? "展开侧边栏" : "折叠侧边栏"}
        >
          <ChevronLeft
            className={cn(
              "w-4 h-4 transition-transform",
              sidebarCollapsed && "rotate-180",
            )}
          />
        </button>
      </div>

      {/* Dual Zone Switcher (Fandom vs Prime) */}
      {!sidebarCollapsed && (
        <div className="p-3 pb-1">
          <div className="grid grid-cols-2 gap-1 p-1 bg-muted/60 rounded-xl border border-border/40 text-xs font-medium">
            {/* Fandom Tab */}
            <button
              type="button"
              onClick={() => setActiveCategory("fandom")}
              className={cn(
                "flex items-center justify-center gap-1.5 py-2 px-2.5 rounded-lg transition-all",
                activeCategory === "fandom"
                  ? "bg-card text-foreground font-semibold shadow-xs"
                  : "text-muted-foreground hover:text-foreground hover:bg-card/40",
              )}
            >
              <img
                src="/icons/fandom.svg"
                alt="Fandom"
                className="w-3.5 h-4 object-contain shrink-0"
              />
              <span>Fandom</span>
              <span className="ml-0.5 px-1.5 py-0.2 text-[10px] rounded-full bg-muted text-muted-foreground font-mono">
                {fandomRooms.length}
              </span>
            </button>

            {/* Prime Chat Tab */}
            <button
              type="button"
              onClick={() => setActiveCategory("prime")}
              className={cn(
                "flex items-center justify-center gap-1.5 py-2 px-2.5 rounded-lg transition-all",
                activeCategory === "prime"
                  ? "bg-card text-foreground font-semibold shadow-xs"
                  : "text-muted-foreground hover:text-foreground hover:bg-card/40",
              )}
            >
              <img
                src="/icons/prime-chat.svg"
                alt="Prime Chat"
                className="w-3.5 h-3.5 object-contain shrink-0"
              />
              <span>Prime Chat</span>
              <span className="ml-0.5 px-1.5 py-0.2 text-[10px] rounded-full bg-muted text-muted-foreground font-mono">
                {primeRooms.length}
              </span>
            </button>
          </div>
        </div>
      )}

      {/* Room List View */}
      <div className="flex-1 overflow-y-auto overflow-x-hidden p-2 space-y-1">
        {displayedRooms.length === 0 ? (
          <div className="py-12 text-center text-xs text-muted-foreground select-none">
            暂无聊天对象
          </div>
        ) : (
          displayedRooms.map((room) => {
            const key = roomKey(room);
            const isSelected =
              selectedRoom && roomKey(selectedRoom) === key;

            // 计算该房间最后一条消息预览与时间（无论对方还是我发送的）
            const roomMsgs = messagesByRoom[key];
            const localLastMsg =
              roomMsgs && roomMsgs.length > 0
                ? roomMsgs[roomMsgs.length - 1]
                : null;

            const previewType = localLastMsg?.type ?? room.lastMessageType;
            const previewContent = localLastMsg?.content ?? room.lastMessageContent;
            const previewText =
              previewType === "image"
                ? "[image]"
                : previewType === "video"
                  ? "[video]"
                  : previewContent || "";

            const previewTime = localLastMsg
              ? localLastMsg.createdAt
              : room.lastMessageTime;

            return (
              <button
                type="button"
                key={key}
                onClick={() => handleRoomClick(room)}
                className={cn(
                  "w-full flex items-center gap-3 p-2.5 rounded-xl text-left transition-all relative group",
                  isSelected
                    ? "bg-sidebar-accent text-sidebar-accent-foreground font-medium shadow-xs"
                    : "text-sidebar-foreground/80 hover:bg-sidebar-accent/50 hover:text-sidebar-foreground",
                  sidebarCollapsed && "justify-center p-2",
                )}
                title={`${room.displayName} (${room.chatRoomId})`}
              >
                {/* Active left indicator */}
                {isSelected && !sidebarCollapsed && (
                  <div className="absolute left-0 top-2 bottom-2 w-1 rounded-r-full bg-primary" />
                )}

                {/* Avatar */}
                <div className="relative shrink-0">
                  <Avatar
                    localUrl={room.avatarLocalUrl}
                    remoteUrl={userProfile?.offlineMode ? undefined : room.avatarUrl}
                    label={room.displayName}
                    className="w-10 h-10 rounded-full object-cover bg-muted ring-1 ring-border/40"
                    fallbackClassName="text-sm text-muted-foreground"
                  />
                </div>

                {/* Info Text (Expanded) */}
                {!sidebarCollapsed && (
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center justify-between gap-1 mb-0.5">
                      <span className="text-xs font-semibold truncate text-foreground">
                        {room.displayName}
                      </span>
                      {previewTime && (
                        <span className="text-[10px] text-muted-foreground shrink-0 font-mono">
                          {formatShortDate(previewTime)}
                        </span>
                      )}
                    </div>
                    <p className="text-[11px] text-muted-foreground truncate line-clamp-1">
                      {previewText}
                    </p>
                  </div>
                )}
              </button>
            );
          })
        )}
      </div>

      {/* Footer Profile & Actions */}
      <div className="p-3 border-t border-sidebar-border bg-sidebar/80 backdrop-blur-xs flex items-center justify-between">
        {!sidebarCollapsed ? (
          <div className="flex items-center gap-2.5 min-w-0">
            <Avatar
              localUrl={userProfile?.avatarLocalUrl}
              remoteUrl={userProfile?.offlineMode ? undefined : userProfile?.avatarUrl}
              label={userProfile?.displayName || "用户"}
              className="w-8 h-8 rounded-full bg-muted ring-1 ring-border/50 shrink-0 object-cover"
              fallbackClassName="text-xs text-muted-foreground"
            />
            <div className="flex flex-col min-w-0">
              <span className="text-xs font-semibold text-foreground truncate">
                {userProfile?.displayName || "？？？"}
              </span>
              <span className="text-[10px] text-muted-foreground truncate">
                {userProfile?.uniqueId ? `@${userProfile.uniqueId}` : "离线浏览模式"}
              </span>
            </div>
          </div>
        ) : (
          <div className="w-full flex justify-center">
            <Avatar
              localUrl={userProfile?.avatarLocalUrl}
              remoteUrl={userProfile?.offlineMode ? undefined : userProfile?.avatarUrl}
              label={userProfile?.displayName || "用户"}
              className="w-8 h-8 rounded-full bg-muted ring-1 ring-border/50 object-cover"
              fallbackClassName="text-xs text-muted-foreground"
            />
          </div>
        )}

        {!sidebarCollapsed && (
          <div className="flex items-center gap-1 shrink-0">
            {/* Quick theme toggle */}
            <button
              type="button"
              onClick={handleThemeToggle}
              className="p-1.5 rounded-lg text-muted-foreground hover:text-foreground hover:bg-sidebar-accent transition-colors"
              title={`当前模式: ${theme === "dark" ? "深色" : "浅色"}，点击切换`}
            >
              {theme === "dark" ? (
                <Moon className="w-4 h-4 text-primary" />
              ) : (
                <Sun className="w-4 h-4 text-amber-500" />
              )}
            </button>

            {/* About modal button */}
            <button
              type="button"
              onClick={() => setAboutModalOpen(true)}
              className="p-1.5 rounded-lg text-muted-foreground hover:text-foreground hover:bg-sidebar-accent transition-colors"
              title="关于 Replive+"
            >
              <Info className="w-4 h-4" />
            </button>
          </div>
        )}
      </div>
    </aside>
  );
};

export default Sidebar;
