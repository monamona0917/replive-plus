import { useEffect } from "react";
import useChatStore from "../../stores/chat-store";
import ChatContainer from "../chat/ChatContainer";
import Watermark from "../chat/Watermark";
import MediaGalleryDrawer from "../drawers/MediaGalleryDrawer";
import SearchDrawer from "../drawers/SearchDrawer";
import AboutModal from "../modals/AboutModal";
import DateJumpModal from "../modals/DateJumpModal";
import MediaLightbox from "../modals/MediaLightbox";
import Sidebar from "./Sidebar";

export const AppLayout = () => {
  const loadRooms = useChatStore((s) => s.loadRooms);
  const loadUserProfile = useChatStore((s) => s.loadUserProfile);
  const selectedRoom = useChatStore((s) => s.selectedRoom);
  const userProfile = useChatStore((s) => s.userProfile);
  const pollNewMessages = useChatStore((s) => s.pollNewMessages);
  const setSearchDrawerOpen = useChatStore((s) => s.setSearchDrawerOpen);

  // 初始化加载房间与用户信息
  useEffect(() => {
    void loadRooms();
    void loadUserProfile();
  }, [loadRooms, loadUserProfile]);

  // 后台轮询当前房间的新消息（每 3 秒增量拉取）
  useEffect(() => {
    if (!selectedRoom || selectedRoom.category === "prime" || userProfile?.offlineMode === true) return;
    const interval = setInterval(() => {
      void pollNewMessages(selectedRoom);
    }, 3000);
    return () => clearInterval(interval);
  }, [selectedRoom, pollNewMessages, userProfile?.offlineMode]);

  // 房间列表单独低频刷新，消息轮询会同步更新已加载房间的摘要。
  useEffect(() => {
    if (!selectedRoom || selectedRoom.category === "prime" || userProfile?.offlineMode === true) return;
    const interval = setInterval(() => {
      void loadRooms();
    }, 15000);
    return () => clearInterval(interval);
  }, [selectedRoom, loadRooms, userProfile?.offlineMode]);

  // 全局快捷键监听 (Ctrl+F 全局搜索)
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "f") {
        e.preventDefault();
        setSearchDrawerOpen(true);
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [setSearchDrawerOpen]);

  return (
    <div className="flex h-screen w-screen overflow-hidden bg-background text-foreground relative">
      {/* 防录屏智能水印 */}
      <Watermark />

      {/* 侧边栏 */}
      <Sidebar />

      {/* 核心主聊天窗口 */}
      <ChatContainer />

      {/* 全屏媒体 Lightbox 浏览器 */}
      <MediaLightbox />

      {/* 侧边搜索抽屉 */}
      <SearchDrawer />

      {/* 侧边媒体相册抽屉 */}
      <MediaGalleryDrawer />

      {/* 日期穿梭弹窗 */}
      <DateJumpModal />

      {/* 关于弹窗 */}
      <AboutModal />
    </div>
  );
};

export default AppLayout;
