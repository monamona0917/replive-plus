import { format } from "date-fns";
import { create } from "zustand";
import type {
  ChatCategory,
  ChatRoom,
  MediaItem,
  Message,
  MessageGroup,
  UserProfile,
} from "../types/chat";
import {
  fetchChatMessages,
  fetchChatRooms,
  fetchRoomAllMedia,
  fetchRoomAvailableDates,
  fetchUserProfile,
  searchChatMessages,
  sendChatMessage,
} from "../utils/fetch-data";

const PAGE_SIZE = 30;

export interface ChatState {
  // 专区与房间
  activeCategory: ChatCategory;
  rooms: ChatRoom[];
  selectedRoom: ChatRoom | null;

  // 消息流状态
  messagesByRoom: Record<string, Message[]>;
  cursorByRoom: Record<string, number>;
  newerCursorByRoom: Record<string, number>;
  hasMoreByRoom: Record<string, boolean>;
  hasNewerByRoom: Record<string, boolean>;
  messageGroups: MessageGroup[];
  mediaList: MediaItem[];
  mediaListByRoom: Record<string, MediaItem[]>;
  isLoadingMedia: boolean;

  // 可用日期列表（用于日历高亮）
  availableDatesByRoom: Record<string, string[]>;
  isLoadingDates: boolean;

  // 搜索与定位
  searchQuery: string;
  searchResults: Message[];
  isSearching: boolean;
  jumpTargetMessageId: string | null;
  scrollToBottomToken: number;

  // 用户状态
  userProfile: UserProfile | null;

  // 弹窗与抽屉状态
  searchDrawerOpen: boolean;
  mediaGalleryDrawerOpen: boolean;
  dateJumpModalOpen: boolean;
  aboutModalOpen: boolean;
  lightboxMedia: MediaItem | null;

  // 加载与错误状态
  isLoadingRooms: boolean;
  isLoadingMessagesByRoom: Record<string, boolean>;
  isLoadingMore: boolean;
  isLoadingNewer: boolean;
  error: string | null;

  // 方法定义
  setActiveCategory: (category: ChatCategory) => void;
  loadUserProfile: () => Promise<void>;
  loadRooms: () => Promise<void>;
  selectRoom: (room: ChatRoom) => Promise<void>;
  loadLatestMessages: (room?: ChatRoom) => Promise<void>;
  loadOlderMessages: (room?: ChatRoom) => Promise<void>;
  loadNewerMessages: (room?: ChatRoom) => Promise<void>;
  loadRoomMedia: (room?: ChatRoom) => Promise<void>;
  loadRoomAvailableDates: (room?: ChatRoom) => Promise<void>;
  jumpToDate: (date: string, room?: ChatRoom) => Promise<void>;
  jumpToMessage: (message: Message, room?: ChatRoom) => Promise<void>;
  searchMessages: (query: string) => Promise<void>;
  clearSearch: () => void;
  sendMessage: (content: string) => Promise<void>;
  pollNewMessages: (room?: ChatRoom) => Promise<void>;
  clearJumpTarget: () => void;
  requestScrollToBottom: () => Promise<void>;
  setError: (error: string | null) => void;

  // 抽屉与弹窗控制
  setSearchDrawerOpen: (open: boolean) => void;
  setMediaGalleryDrawerOpen: (open: boolean) => void;
  setDateJumpModalOpen: (open: boolean) => void;
  setAboutModalOpen: (open: boolean) => void;
  openLightbox: (media: MediaItem) => void;
  closeLightbox: () => void;
  stepLightbox: (delta: number) => void;
}

export function roomKey(room: ChatRoom): string {
  return `${room.category || "fandom"}:${room.chatRoomId || room.displayName || room.userId}`;
}

export function groupMessagesByDate(messages: Message[]): MessageGroup[] {
  const groups: MessageGroup[] = [];
  let currentGroup: MessageGroup | null = null;

  for (const message of messages) {
    let dateStr = "未知日期";
    try {
      dateStr = format(new Date(message.createdAt), "yyyy-MM-dd");
    } catch {
      dateStr = message.createdAt.slice(0, 10) || "未知日期";
    }

    if (!currentGroup || currentGroup.date !== dateStr) {
      if (currentGroup) groups.push(currentGroup);
      currentGroup = {
        date: dateStr,
        messages: [message],
      };
    } else {
      currentGroup.messages.push(message);
    }
  }

  if (currentGroup) groups.push(currentGroup);
  return groups;
}

function extractMediaItems(messages: Message[]): MediaItem[] {
  const items: MediaItem[] = [];
  for (const msg of messages) {
    if ((msg.type === "image" || msg.type === "video") && msg.mediaUrl) {
      items.push({
        id: `media-${msg.id}`,
        type: msg.type,
        url: msg.mediaUrl,
        createdAt: msg.createdAt,
        senderName: msg.senderName,
        messageId: msg.id,
        backendId: msg.backendId,
      });
    }
  }
  return items;
}

function extractDatesFromMessages(messages: Message[]): string[] {
  const dates = new Set<string>();
  for (const msg of messages) {
    try {
      const d = format(new Date(msg.createdAt), "yyyy-MM-dd");
      if (d) dates.add(d);
    } catch {
      const sliceD = msg.createdAt.slice(0, 10);
      if (sliceD && /^\d{4}-\d{2}-\d{2}$/.test(sliceD)) {
        dates.add(sliceD);
      }
    }
  }
  return Array.from(dates).sort();
}

function uniqueSortedMessages(messages: Message[]): Message[] {
  const byId = new Map<string, Message>();

  for (const message of messages) {
    const key = message.chatMessageId || String(message.backendId) || message.id;
    byId.set(key, message);
  }

  return Array.from(byId.values()).sort((a, b) => {
    const timeDiff = new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime();
    if (timeDiff !== 0) return timeDiff;
    return a.backendId - b.backendId;
  });
}

export const useChatStore = create<ChatState>((set, get) => ({
  activeCategory: "fandom",
  rooms: [],
  selectedRoom: null,

  messagesByRoom: {},
  cursorByRoom: {},
  newerCursorByRoom: {},
  hasMoreByRoom: {},
  hasNewerByRoom: {},
  messageGroups: [],
  mediaList: [],
  mediaListByRoom: {},
  isLoadingMedia: false,

  availableDatesByRoom: {},
  isLoadingDates: false,

  searchQuery: "",
  searchResults: [],
  isSearching: false,
  jumpTargetMessageId: null,
  scrollToBottomToken: 0,

  userProfile: null,

  searchDrawerOpen: false,
  mediaGalleryDrawerOpen: false,
  dateJumpModalOpen: false,
  aboutModalOpen: false,
  lightboxMedia: null,

  isLoadingRooms: false,
  isLoadingMessagesByRoom: {},
  isLoadingMore: false,
  isLoadingNewer: false,
  error: null,

  setActiveCategory: (category: ChatCategory) => {
    set({ activeCategory: category });
    const { selectedRoom, rooms } = get();
    if (!selectedRoom || selectedRoom.category !== category) {
      const firstRoomInCat = rooms.find((r) => (r.category || "fandom") === category);
      if (firstRoomInCat) {
        void get().selectRoom(firstRoomInCat);
      }
    }
  },

  loadUserProfile: async () => {
    try {
      const profile = await fetchUserProfile();
      set({ userProfile: profile });
    } catch {
      // 保持默认
    }
  },

  loadRooms: async () => {
    if (get().isLoadingRooms) return;
    set({ isLoadingRooms: true, error: null });

    try {
      const rooms = await fetchChatRooms();
      set({ rooms, isLoadingRooms: false });

      const currentRoom = get().selectedRoom;
      const currentCategory = get().activeCategory;

      if (!currentRoom && rooms.length > 0) {
        const firstInCat =
          rooms.find((r) => (r.category || "fandom") === currentCategory) ||
          rooms[0];
        if (firstInCat) {
          if (firstInCat.category && firstInCat.category !== currentCategory) {
            set({ activeCategory: firstInCat.category });
          }
          await get().selectRoom(firstInCat);
        }
      }
    } catch (err) {
      set({
        error: err instanceof Error ? err.message : "加载聊天对象列表失败",
        isLoadingRooms: false,
  isLoadingMessagesByRoom: {},
      });
    }
  },

  selectRoom: async (room: ChatRoom) => {
    const key = roomKey(room);
    const roomCat = room.category || "fandom";

    set((state) => {
      const messages = state.messagesByRoom[key] ?? [];
      const currentMedia =
        state.mediaListByRoom[key] ?? extractMediaItems(messages);
      return {
        selectedRoom: room,
        activeCategory: roomCat,
        searchQuery: "",
        searchResults: [],
        messageGroups: groupMessagesByDate(messages),
        mediaList: currentMedia,
        error: null,
      };
    });

    // 用户可能此前停留在历史或日期跳转页；每次主动进入房间均重置到最新页。
    await get().loadLatestMessages(room);

    const selectedRoom = get().selectedRoom;
    if (selectedRoom && roomKey(selectedRoom) === key) {
      set((state) => ({
        scrollToBottomToken: state.scrollToBottomToken + 1,
      }));
    }

    // 后台预加载该会话的全部媒体与日期列表
    void get().loadRoomMedia(room);
    void get().loadRoomAvailableDates(room);
  },

  loadRoomAvailableDates: async (room?: ChatRoom) => {
    const targetRoom = room ?? get().selectedRoom;
    if (!targetRoom) return;

    const key = roomKey(targetRoom);
    set({ isLoadingDates: true });

    try {
      const datesFromApi = await fetchRoomAvailableDates(targetRoom);
      const localDates = extractDatesFromMessages(
        get().messagesByRoom[key] ?? [],
      );

      set((state) => ({
        availableDatesByRoom: {
          ...state.availableDatesByRoom,
          [key]: Array.from(
            new Set([
              ...(state.availableDatesByRoom[key] ?? []),
              ...datesFromApi,
              ...localDates,
            ]),
          ).sort(),
        },
        isLoadingDates: false,
      }));
    } catch {
      set({ isLoadingDates: false });
    }
  },

  loadRoomMedia: async (room?: ChatRoom) => {
    const targetRoom = room ?? get().selectedRoom;
    if (!targetRoom) return;

    const key = roomKey(targetRoom);
    set({ isLoadingMedia: true });

    try {
      const fullMedia = await fetchRoomAllMedia(targetRoom);
      const localExtracted = extractMediaItems(
        get().messagesByRoom[key] ?? [],
      );

      // 合并去重
      const byUrl = new Map<string, MediaItem>();
      for (const item of [...fullMedia, ...localExtracted]) {
        byUrl.set(item.url, item);
      }
      const mergedMedia = Array.from(byUrl.values()).sort(
        (a, b) =>
          new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime(),
      );

      set((state) => ({
        mediaListByRoom: {
          ...state.mediaListByRoom,
          [key]: mergedMedia,
        },
        mediaList: mergedMedia,
        isLoadingMedia: false,
      }));
    } catch {
      set({ isLoadingMedia: false });
    }
  },

  loadLatestMessages: async (room?: ChatRoom) => {
    const targetRoom = room ?? get().selectedRoom;
    if (!targetRoom) return;

    const key = roomKey(targetRoom);
    if (get().isLoadingMessagesByRoom[key]) return;

    set((state) => ({
      isLoadingMessagesByRoom: {
        ...state.isLoadingMessagesByRoom,
        [key]: true,
      },
      error: null,
    }));

    try {
      const page = await fetchChatMessages({
        room: targetRoom,
        pageSize: PAGE_SIZE,
      });
      const pageMessages = uniqueSortedMessages(page.messages);

      set((state) => {
        const pendingMessages = (state.messagesByRoom[key] ?? []).filter(
          (message) => message.backendId === 0,
        );
        // 服务端记录排在后面，以正式记录替换同 ID 的本地待同步消息。
        const sorted = uniqueSortedMessages([...pendingMessages, ...pageMessages]);
        const localMedia = extractMediaItems(sorted);
        const localDates = extractDatesFromMessages(sorted);
        const existingMedia = state.mediaListByRoom[key] || [];
        const byUrl = new Map<string, MediaItem>();
        for (const item of [...existingMedia, ...localMedia]) {
          byUrl.set(item.url, item);
        }
        const mergedMedia = Array.from(byUrl.values()).sort(
          (a, b) =>
            new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime(),
        );

        const existingDates = state.availableDatesByRoom[key] || [];
        const mergedDates = Array.from(
          new Set([...existingDates, ...localDates]),
        ).sort();
        const selectedKey = state.selectedRoom ? roomKey(state.selectedRoom) : "";

        return {
          messagesByRoom: {
            ...state.messagesByRoom,
            [key]: sorted,
          },
          cursorByRoom: {
            ...state.cursorByRoom,
            [key]: page.prevCursorId || page.nextCursorId,
          },
          newerCursorByRoom: {
            ...state.newerCursorByRoom,
            [key]: page.nextCursorId,
          },
          hasMoreByRoom: {
            ...state.hasMoreByRoom,
            [key]: page.hasOlder,
          },
          hasNewerByRoom: {
            ...state.hasNewerByRoom,
            [key]: page.hasNewer,
          },
          ...(selectedKey === key ? { mediaList: mergedMedia } : {}),
          mediaListByRoom: {
            ...state.mediaListByRoom,
            [key]: mergedMedia,
          },
          availableDatesByRoom: {
            ...state.availableDatesByRoom,
            [key]: mergedDates,
          },
          isLoadingMessagesByRoom: {
            ...state.isLoadingMessagesByRoom,
            [key]: false,
          },
          jumpTargetMessageId: null,
        };
      });
    } catch (err) {
      set((state) => ({
        error: err instanceof Error ? err.message : "Failed to load chat messages",
        isLoadingMessagesByRoom: {
          ...state.isLoadingMessagesByRoom,
          [key]: false,
        },
      }));
    }
  },

  loadOlderMessages: async (room?: ChatRoom) => {
    const targetRoom = room ?? get().selectedRoom;
    if (!targetRoom || get().isLoadingMore) return;

    const key = roomKey(targetRoom);
    const hasMore = get().hasMoreByRoom[key] ?? false;
    const cursorId = get().cursorByRoom[key] ?? 0;
    if (!hasMore || cursorId <= 0) return;

    set({ isLoadingMore: true, error: null });
    try {
      const page = await fetchChatMessages({ room: targetRoom, cursorId, pageSize: PAGE_SIZE });

      set((state) => {
        const existing = state.messagesByRoom[key] ?? [];
        const merged = uniqueSortedMessages([...page.messages, ...existing]);
        const addedDates = extractDatesFromMessages(page.messages);
        const existingDates = state.availableDatesByRoom[key] || [];
        const mergedDates = Array.from(
          new Set([...existingDates, ...addedDates]),
        ).sort();

        return {
          messagesByRoom: {
            ...state.messagesByRoom,
            [key]: merged,
          },
          cursorByRoom: {
            ...state.cursorByRoom,
            [key]: page.prevCursorId || page.nextCursorId,
          },
          hasMoreByRoom: {
            ...state.hasMoreByRoom,
            [key]: page.hasOlder,
          },
          availableDatesByRoom: {
            ...state.availableDatesByRoom,
            [key]: mergedDates,
          },
          messageGroups: groupMessagesByDate(merged),
          isLoadingMore: false,
        };
      });
    } catch (err) {
      set({
        error: err instanceof Error ? err.message : "加载历史消息失败",
        isLoadingMore: false,
      });
    }
  },

  loadNewerMessages: async (room?: ChatRoom) => {
    const targetRoom = room ?? get().selectedRoom;
    if (!targetRoom || get().isLoadingNewer) return;

    const key = roomKey(targetRoom);
    const hasNewer = get().hasNewerByRoom[key] ?? false;
    const cursorId = get().newerCursorByRoom[key] ?? 0;
    if (!hasNewer || cursorId <= 0) return;

    set({ isLoadingNewer: true, error: null });
    try {
      const page = await fetchChatMessages({
        room: targetRoom,
        cursorId,
        direction: "newer",
        pageSize: PAGE_SIZE,
      });

      set((state) => {
        const existing = state.messagesByRoom[key] ?? [];
        const merged = uniqueSortedMessages([...existing, ...page.messages]);
        const addedDates = extractDatesFromMessages(page.messages);
        const existingDates = state.availableDatesByRoom[key] || [];
        const mergedDates = Array.from(
          new Set([...existingDates, ...addedDates]),
        ).sort();

        return {
          messagesByRoom: {
            ...state.messagesByRoom,
            [key]: merged,
          },
          newerCursorByRoom: {
            ...state.newerCursorByRoom,
            [key]: page.nextCursorId,
          },
          hasNewerByRoom: {
            ...state.hasNewerByRoom,
            [key]: page.hasNewer,
          },
          hasMoreByRoom: {
            ...state.hasMoreByRoom,
            [key]: page.hasOlder,
          },
          availableDatesByRoom: {
            ...state.availableDatesByRoom,
            [key]: mergedDates,
          },
          messageGroups: groupMessagesByDate(merged),
          isLoadingNewer: false,
        };
      });
    } catch (err) {
      set({
        error: err instanceof Error ? err.message : "加载后续消息失败",
        isLoadingNewer: false,
      });
    }
  },

  jumpToDate: async (date: string, room?: ChatRoom) => {
    const targetRoom = room ?? get().selectedRoom;
    if (!targetRoom) return;

    const key = roomKey(targetRoom);
    if (get().isLoadingMessagesByRoom[key]) return;

    set((state) => ({
      isLoadingMessagesByRoom: {
        ...state.isLoadingMessagesByRoom,
        [key]: true,
      },
      error: null,
    }));

    try {
      const page = await fetchChatMessages({
        room: targetRoom,
        date,
        direction: "around",
        pageSize: PAGE_SIZE,
      });
      const targetMessageId = page.messages[0]?.id ?? null;
      const sorted = uniqueSortedMessages(page.messages);

      set((state) => ({
        messagesByRoom: {
          ...state.messagesByRoom,
          [key]: sorted,
        },
        cursorByRoom: {
          ...state.cursorByRoom,
          [key]: page.prevCursorId,
        },
        newerCursorByRoom: {
          ...state.newerCursorByRoom,
          [key]: page.nextCursorId,
        },
        hasMoreByRoom: {
          ...state.hasMoreByRoom,
          [key]: page.hasOlder,
        },
        hasNewerByRoom: {
          ...state.hasNewerByRoom,
          [key]: page.hasNewer,
        },
        isLoadingMessagesByRoom: {
          ...state.isLoadingMessagesByRoom,
          [key]: false,
        },
        jumpTargetMessageId: targetMessageId,
        dateJumpModalOpen: false,
      }));
    } catch (err) {
      set((state) => ({
        error: err instanceof Error ? err.message : "Failed to load messages for this date",
        isLoadingMessagesByRoom: {
          ...state.isLoadingMessagesByRoom,
          [key]: false,
        },
      }));
    }
  },

  jumpToMessage: async (message: Message, room?: ChatRoom) => {
    const targetRoom = room ?? get().selectedRoom;
    if (!targetRoom) return;

    const key = roomKey(targetRoom);
    if (get().isLoadingMessagesByRoom[key]) return;

    set((state) => ({
      isLoadingMessagesByRoom: {
        ...state.isLoadingMessagesByRoom,
        [key]: true,
      },
      error: null,
    }));

    try {
      const page = await fetchChatMessages({
        room: targetRoom,
        anchorId: message.backendId,
        direction: "around",
        pageSize: PAGE_SIZE,
      });
      const sorted = uniqueSortedMessages(page.messages);

      set((state) => ({
        messagesByRoom: {
          ...state.messagesByRoom,
          [key]: sorted,
        },
        cursorByRoom: {
          ...state.cursorByRoom,
          [key]: page.prevCursorId,
        },
        newerCursorByRoom: {
          ...state.newerCursorByRoom,
          [key]: page.nextCursorId,
        },
        hasMoreByRoom: {
          ...state.hasMoreByRoom,
          [key]: page.hasOlder,
        },
        hasNewerByRoom: {
          ...state.hasNewerByRoom,
          [key]: page.hasNewer,
        },
        isLoadingMessagesByRoom: {
          ...state.isLoadingMessagesByRoom,
          [key]: false,
        },
        jumpTargetMessageId: message.id,
        searchDrawerOpen: false,
        mediaGalleryDrawerOpen: false,
      }));
    } catch (err) {
      set((state) => ({
        error: err instanceof Error ? err.message : "Failed to jump to message",
        isLoadingMessagesByRoom: {
          ...state.isLoadingMessagesByRoom,
          [key]: false,
        },
      }));
    }
  },

  searchMessages: async (query: string) => {
    const targetRoom = get().selectedRoom;
    const keyword = query.trim();
    if (!targetRoom || !keyword) {
      set({ searchResults: [], isSearching: false, searchQuery: "" });
      return;
    }

    set({ isSearching: true, error: null, searchQuery: keyword });
    try {
      const page = await searchChatMessages({ room: targetRoom, keyword, pageSize: 50 });

      let results = page.messages;
      if (results.length === 0) {
        const localMessages = get().messagesByRoom[roomKey(targetRoom)] ?? [];
        results = localMessages.filter((m) =>
          m.content.toLowerCase().includes(keyword.toLowerCase()),
        );
      }

      set({ searchResults: results, isSearching: false });
    } catch (err) {
      set({
        error: err instanceof Error ? err.message : "搜索消息失败",
        isSearching: false,
      });
    }
  },

  clearSearch: () => {
    set({ searchQuery: "", searchResults: [], isSearching: false });
  },

  sendMessage: async (content: string) => {
    const room = get().selectedRoom;
    if (!room || !content.trim()) return;
    if (room.category === "prime") {
      throw new Error("Prime Chat messages are read-only in this application");
    }

    const trimmed = content.trim();
    try {
      const chatMessageId = await sendChatMessage({
        chatRoomId: room.chatRoomId,
        content: trimmed,
      });

      const key = roomKey(room);
      const now = new Date().toISOString();
      const localMsg: Message = {
        // 使用后端回传的正式 ID，服务端同步后会替换待同步消息，而非新增气泡。
        id: chatMessageId,
        backendId: 0,
        chatMessageId,
        content: trimmed,
        type: "text",
        createdAt: now,
        senderId: get().userProfile?.userId || "user_me",
        senderName: get().userProfile?.displayName || "我",
        senderKind: "member",
      };

      set((state) => {
        const existing = state.messagesByRoom[key] ?? [];
        const merged = uniqueSortedMessages([...existing, localMsg]);
        return {
          messagesByRoom: {
            ...state.messagesByRoom,
            [key]: merged,
          },
          messageGroups: groupMessagesByDate(merged),
          scrollToBottomToken: state.scrollToBottomToken + 1,
        };
      });
    } catch (err) {
      throw new Error(err instanceof Error ? err.message : "发送消息失败");
    }
  },

  pollNewMessages: async (room?: ChatRoom) => {
    const targetRoom = room ?? get().selectedRoom;
    if (!targetRoom || targetRoom.category === "prime") return;

    const key = roomKey(targetRoom);
    const existing = get().messagesByRoom[key] ?? [];
    if (existing.length === 0) return;

    const serverMessages = existing.filter(
      (message) =>
        message.backendId > 0 &&
        !message.id.startsWith("local-") &&
        !message.chatMessageId.startsWith("local-"),
    );
    const newestMessage = serverMessages.reduce<Message | null>((latest, message) => {
      if (!latest) return message;
      const latestTime = new Date(latest.createdAt).getTime();
      const messageTime = new Date(message.createdAt).getTime();
      return messageTime > latestTime || (messageTime === latestTime && message.backendId > latest.backendId)
        ? message
        : latest;
    }, null);
    if (!newestMessage || newestMessage.backendId <= 0) return;

    try {
      const page = await fetchChatMessages({
        room: targetRoom,
        cursorId: newestMessage.backendId,
        direction: "newer",
        pageSize: 50,
      });

      if (page.messages.length === 0) return;

      set((state) => {
        const merged = uniqueSortedMessages([...existing, ...page.messages]);
        return {
          messagesByRoom: {
            ...state.messagesByRoom,
            [key]: merged,
          },
          newerCursorByRoom: {
            ...state.newerCursorByRoom,
            [key]: page.nextCursorId,
          },
          hasNewerByRoom: {
            ...state.hasNewerByRoom,
            [key]: page.hasNewer,
          },
          messageGroups: groupMessagesByDate(merged),
        };
      });
    } catch {
      // 轮询静默失败
    }
  },

  clearJumpTarget: () => {
    set({ jumpTargetMessageId: null });
  },

  requestScrollToBottom: async () => {
    const targetRoom = get().selectedRoom;
    if (!targetRoom) return;
    if (get().hasNewerByRoom[roomKey(targetRoom)]) {
      await get().loadLatestMessages(targetRoom);
    }
    set((state) => ({ scrollToBottomToken: state.scrollToBottomToken + 1 }));
  },

  setError: (error: string | null) => {
    set({ error });
  },

  setSearchDrawerOpen: (open: boolean) => {
    set({ searchDrawerOpen: open });
    if (!open) {
      get().clearSearch();
    }
  },

  setMediaGalleryDrawerOpen: (open: boolean) => {
    set({ mediaGalleryDrawerOpen: open });
    if (open) {
      void get().loadRoomMedia(get().selectedRoom || undefined);
    }
  },

  setDateJumpModalOpen: (open: boolean) => {
    set({ dateJumpModalOpen: open });
    if (open) {
      void get().loadRoomAvailableDates(get().selectedRoom || undefined);
    }
  },

  setAboutModalOpen: (open: boolean) => {
    set({ aboutModalOpen: open });
  },

  openLightbox: (media: MediaItem) => {
    set({ lightboxMedia: media });
  },

  closeLightbox: () => {
    set({ lightboxMedia: null });
  },

  stepLightbox: (delta: number) => {
    const { lightboxMedia, mediaList } = get();
    if (!lightboxMedia || mediaList.length === 0) return;
    const currentIndex = mediaList.findIndex((m) => m.id === lightboxMedia.id);
    if (currentIndex === -1) return;

    let nextIndex = currentIndex + delta;
    if (nextIndex < 0) nextIndex = mediaList.length - 1;
    if (nextIndex >= mediaList.length) nextIndex = 0;

    set({ lightboxMedia: mediaList[nextIndex] });
  },
}));

export default useChatStore;
