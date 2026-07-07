import { format } from "date-fns";
import { create } from "zustand";
import type {
  ChatRoom,
  ChatStats,
  Message,
  MessageGroup,
  TranslationState,
} from "../types/chat";
import {
  fetchChatMessages,
  fetchChatRooms,
  searchChatMessages,
  sendChatMessage,
  translateText,
} from "../utils/fetch-data";

const PAGE_SIZE = 30;

interface ChatState {
  rooms: ChatRoom[];
  selectedRoom: ChatRoom | null;
  messagesByRoom: Record<string, Message[]>;
  cursorByRoom: Record<string, number>;
  newerCursorByRoom: Record<string, number>;
  hasMoreByRoom: Record<string, boolean>;
  hasNewerByRoom: Record<string, boolean>;
  messageGroups: MessageGroup[];
  searchQuery: string;
  searchResults: Message[];
  selectedDate: Date | null;
  isLoadingRooms: boolean;
  isLoadingMessages: boolean;
  isLoadingMore: boolean;
  isLoadingNewer: boolean;
  isSearching: boolean;
  error: string | null;
  stats: ChatStats | null;
  translationByMessageId: Record<string, TranslationState>;
  jumpTargetMessageId: string | null;
  scrollToBottomToken: number;

  loadRooms: () => Promise<void>;
  selectRoom: (room: ChatRoom) => Promise<void>;
  loadLatestMessages: (room?: ChatRoom) => Promise<void>;
  loadOlderMessages: (room?: ChatRoom) => Promise<void>;
  loadNewerMessages: (room?: ChatRoom) => Promise<void>;
  jumpToDate: (date: string, room?: ChatRoom) => Promise<void>;
  jumpToMessage: (message: Message, room?: ChatRoom) => Promise<void>;
  toggleTranslation: (message: Message) => Promise<void>;
  setSearchQuery: (query: string) => void;
  setSelectedDate: (date: Date | null) => void;
  setError: (error: string | null) => void;
  searchMessages: (query: string) => Promise<void>;
  clearJumpTarget: () => void;
  requestScrollToBottom: () => Promise<void>;
  sendMessage: (content: string) => Promise<void>;
  clearSearch: () => void;
}

function roomKey(room: ChatRoom) {
  return room.chatRoomId || room.displayName;
}

function selectedMessages(state: ChatState) {
  if (!state.selectedRoom) return [];
  return state.messagesByRoom[roomKey(state.selectedRoom)] ?? [];
}

function groupMessagesByDate(messages: Message[]) {
  const groups: MessageGroup[] = [];
  let currentGroup: MessageGroup | null = null;

  for (const message of messages) {
    const messageDate = format(new Date(message.createdAt), "yyyy-MM-dd");

    if (!currentGroup || currentGroup.date !== messageDate) {
      if (currentGroup) groups.push(currentGroup);
      currentGroup = {
        date: messageDate,
        messages: [message],
      };
    } else {
      currentGroup.messages.push(message);
    }
  }

  if (currentGroup) groups.push(currentGroup);
  return groups;
}

function generateStats(messages: Message[]): ChatStats | null {
  if (messages.length === 0) return null;

  const messagesByDate: Record<string, number> = {};
  let imageCount = 0;
  let videoCount = 0;

  for (const message of messages) {
    const date = format(new Date(message.createdAt), "yyyy-MM-dd");
    messagesByDate[date] = (messagesByDate[date] || 0) + 1;

    if (message.type === "image") imageCount++;
    if (message.type === "video") videoCount++;
  }

  const timestamps = messages.map((message) =>
    new Date(message.createdAt).getTime(),
  );

  return {
    totalMessages: messages.length,
    messagesByDate,
    mediaCount: {
      images: imageCount,
      videos: videoCount,
    },
    firstMessageDate: format(new Date(Math.min(...timestamps)), "yyyy-MM-dd"),
    lastMessageDate: format(new Date(Math.max(...timestamps)), "yyyy-MM-dd"),
  };
}

function uniqueSortedMessages(messages: Message[]) {
  const byId = new Map<string, Message>();

  for (const message of messages) {
    const key = message.chatMessageId || String(message.backendId);
    byId.set(key, message);
  }

  return Array.from(byId.values()).sort((a, b) => {
    if (a.backendId !== b.backendId) return a.backendId - b.backendId;
    return new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime();
  });
}

function buildCurrentDerivedState(state: ChatState) {
  const messages = selectedMessages(state);
  return {
    messageGroups: groupMessagesByDate(messages),
    stats: generateStats(messages),
    searchResults: state.searchQuery.trim()
      ? searchLoadedMessages(messages, state.searchQuery)
      : [],
  };
}

function searchLoadedMessages(messages: Message[], query: string) {
  const lowerQuery = query.trim().toLowerCase();
  if (!lowerQuery) return [];

  return messages.filter((message) =>
    message.content.toLowerCase().includes(lowerQuery),
  );
}

const useChatStore = create<ChatState>((set, get) => ({
  rooms: [],
  selectedRoom: null,
  messagesByRoom: {},
  cursorByRoom: {},
  newerCursorByRoom: {},
  hasMoreByRoom: {},
  hasNewerByRoom: {},
  messageGroups: [],
  searchQuery: "",
  searchResults: [],
  selectedDate: null,
  isLoadingRooms: false,
  isLoadingMessages: false,
  isLoadingMore: false,
  isLoadingNewer: false,
  isSearching: false,
  error: null,
  stats: null,
  translationByMessageId: {},
  jumpTargetMessageId: null,
  scrollToBottomToken: 0,

  loadRooms: async () => {
    if (get().isLoadingRooms) return;

    set({ isLoadingRooms: true, error: null });
    try {
      const rooms = await fetchChatRooms();
      set({ rooms, isLoadingRooms: false });

      const currentRoom = get().selectedRoom;
      if (rooms.length > 0 && !currentRoom) {
        await get().selectRoom(rooms[0]);
      }
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : "加载聊天对象失败",
        isLoadingRooms: false,
      });
    }
  },

  selectRoom: async (room) => {
    const key = roomKey(room);
    set((state) => ({
      selectedRoom: room,
      searchQuery: "",
      searchResults: [],
      messageGroups: groupMessagesByDate(state.messagesByRoom[key] ?? []),
      stats: generateStats(state.messagesByRoom[key] ?? []),
      error: null,
    }));

    if (!get().messagesByRoom[key]) {
      await get().loadLatestMessages(room);
    }
  },

  loadLatestMessages: async (room) => {
    const targetRoom = room ?? get().selectedRoom;
    if (!targetRoom || get().isLoadingMessages) return;

    set({ isLoadingMessages: true, error: null });
    try {
      const page = await fetchChatMessages({
        displayName: targetRoom.displayName,
        pageSize: PAGE_SIZE,
      });
      const key = roomKey(targetRoom);

      set((state) => {
        const nextState = {
          ...state,
          messagesByRoom: {
            ...state.messagesByRoom,
            [key]: uniqueSortedMessages(page.messages),
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
          isLoadingMessages: false,
          jumpTargetMessageId: null,
        };

        return {
          ...nextState,
          ...buildCurrentDerivedState(nextState),
        };
      });
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : "加载消息失败",
        isLoadingMessages: false,
      });
    }
  },

  loadOlderMessages: async (room) => {
    const targetRoom = room ?? get().selectedRoom;
    if (!targetRoom || get().isLoadingMore) return;

    const key = roomKey(targetRoom);
    const hasMore = get().hasMoreByRoom[key] ?? false;
    const cursorId = get().cursorByRoom[key] ?? 0;
    if (!hasMore || cursorId <= 0) return;

    set({ isLoadingMore: true, error: null });
    try {
      const page = await fetchChatMessages({
        displayName: targetRoom.displayName,
        cursorId,
        pageSize: PAGE_SIZE,
      });

      set((state) => {
        const existingMessages = state.messagesByRoom[key] ?? [];
        const nextState = {
          ...state,
          messagesByRoom: {
            ...state.messagesByRoom,
            [key]: uniqueSortedMessages([...page.messages, ...existingMessages]),
          },
          cursorByRoom: {
            ...state.cursorByRoom,
            [key]: page.prevCursorId || page.nextCursorId,
          },
          hasMoreByRoom: {
            ...state.hasMoreByRoom,
            [key]: page.hasOlder,
          },
          isLoadingMore: false,
        };

        return {
          ...nextState,
          ...buildCurrentDerivedState(nextState),
        };
      });
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : "加载历史消息失败",
        isLoadingMore: false,
      });
    }
  },

  loadNewerMessages: async (room) => {
    const targetRoom = room ?? get().selectedRoom;
    if (!targetRoom || get().isLoadingNewer) return;

    const key = roomKey(targetRoom);
    const hasNewer = get().hasNewerByRoom[key] ?? false;
    const cursorId = get().newerCursorByRoom[key] ?? 0;
    if (!hasNewer || cursorId <= 0) return;

    set({ isLoadingNewer: true, error: null });
    try {
      const page = await fetchChatMessages({
        displayName: targetRoom.displayName,
        cursorId,
        direction: "newer",
        pageSize: PAGE_SIZE,
      });

      set((state) => {
        const existingMessages = state.messagesByRoom[key] ?? [];
        const nextState = {
          ...state,
          messagesByRoom: {
            ...state.messagesByRoom,
            [key]: uniqueSortedMessages([...existingMessages, ...page.messages]),
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
          isLoadingNewer: false,
        };

        return {
          ...nextState,
          ...buildCurrentDerivedState(nextState),
        };
      });
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : "加载后续消息失败",
        isLoadingNewer: false,
      });
    }
  },

  jumpToDate: async (date, room) => {
    const targetRoom = room ?? get().selectedRoom;
    if (!targetRoom || get().isLoadingMessages) return;

    set({ isLoadingMessages: true, error: null, selectedDate: new Date(date) });
    try {
      const page = await fetchChatMessages({
        displayName: targetRoom.displayName,
        date,
        direction: "around",
        pageSize: PAGE_SIZE,
      });
      const key = roomKey(targetRoom);
      const targetMessageId = page.messages[0]?.id ?? null;

      set((state) => {
        const nextState = {
          ...state,
          messagesByRoom: {
            ...state.messagesByRoom,
            [key]: uniqueSortedMessages(page.messages),
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
          isLoadingMessages: false,
          jumpTargetMessageId: targetMessageId,
        };

        return {
          ...nextState,
          ...buildCurrentDerivedState(nextState),
        };
      });
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : "按日期加载消息失败",
        isLoadingMessages: false,
      });
    }
  },

  jumpToMessage: async (message, room) => {
    const targetRoom = room ?? get().selectedRoom;
    if (!targetRoom || get().isLoadingMessages) return;

    set({ isLoadingMessages: true, error: null });
    try {
      const page = await fetchChatMessages({
        displayName: targetRoom.displayName,
        anchorId: message.backendId,
        direction: "around",
        pageSize: PAGE_SIZE,
      });
      const key = roomKey(targetRoom);

      set((state) => {
        const nextState = {
          ...state,
          messagesByRoom: {
            ...state.messagesByRoom,
            [key]: uniqueSortedMessages(page.messages),
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
          searchQuery: "",
          searchResults: [],
          isLoadingMessages: false,
          jumpTargetMessageId: message.id,
        };

        return {
          ...nextState,
          ...buildCurrentDerivedState(nextState),
        };
      });
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : "跳转消息失败",
        isLoadingMessages: false,
      });
    }
  },

  toggleTranslation: async (message) => {
    const key = message.id;
    const current = get().translationByMessageId[key];

    if (message.type !== "text" || !message.content.trim()) return;
    if (current?.loading) return;
    if (current?.text && current.visible) {
      set((state) => ({
        translationByMessageId: {
          ...state.translationByMessageId,
          [key]: {
            ...current,
            visible: false,
          },
        },
      }));
      return;
    }
    if (current?.text && !current.visible) {
      set((state) => ({
        translationByMessageId: {
          ...state.translationByMessageId,
          [key]: {
            ...current,
            visible: true,
          },
        },
      }));
      return;
    }

    set((state) => ({
      translationByMessageId: {
        ...state.translationByMessageId,
        [key]: {
          loading: true,
          visible: true,
        },
      },
    }));

    try {
      const translatedText = await translateText({ text: message.content });
      set((state) => ({
        translationByMessageId: {
          ...state.translationByMessageId,
          [key]: {
            text: translatedText,
            loading: false,
            visible: true,
          },
        },
      }));
    } catch (error) {
      set((state) => ({
        translationByMessageId: {
          ...state.translationByMessageId,
          [key]: {
            error: error instanceof Error ? error.message : "翻译失败",
            loading: false,
            visible: true,
          },
        },
      }));
    }
  },

  setSearchQuery: (query) => {
    set({
      searchQuery: query,
      searchResults: query.trim() ? get().searchResults : [],
    });
  },

  setSelectedDate: (date) => set({ selectedDate: date }),
  setError: (error) => set({ error }),

  searchMessages: async (query) => {
    const targetRoom = get().selectedRoom;
    const keyword = query.trim();
    if (!targetRoom || !keyword) {
      set({ searchResults: [], isSearching: false });
      return;
    }

    set({ isSearching: true, error: null, searchQuery: keyword });
    try {
      const page = await searchChatMessages({
        displayName: targetRoom.displayName,
        keyword,
        pageSize: 50,
      });
      set({ searchResults: page.messages, isSearching: false });
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : "搜索消息失败",
        isSearching: false,
      });
    }
  },

  clearJumpTarget: () => {
    set({ jumpTargetMessageId: null });
  },

  requestScrollToBottom: async () => {
    const targetRoom = get().selectedRoom;
    if (!targetRoom || get().isLoadingMessages) return;

    await get().loadLatestMessages(targetRoom);
    set((state) => ({ scrollToBottomToken: state.scrollToBottomToken + 1 }));
  },

  sendMessage: async (content) => {
    const room = get().selectedRoom;
    if (!room || !content.trim()) return;

    try {
      await sendChatMessage({
        chatRoomId: room.chatRoomId,
        content: content.trim(),
      });

      // 发送成功后，立即将消息加入本地 state 以便即时显示
      // 后端 sync worker 稍后会从 Replive 同步回来，保证持久化
      const key = roomKey(room);
      const now = new Date().toISOString();
      const localMsg: Message = {
        id: `local-${Date.now()}`,
        backendId: Date.now(),
        chatMessageId: `local-${Date.now()}`,
        content: content.trim(),
        type: "text",
        createdAt: now,
        senderId: "",
        senderName: "我",
      };

      set((state) => {
        const existing = state.messagesByRoom[key] ?? [];
        return {
          messagesByRoom: {
            ...state.messagesByRoom,
            [key]: [...existing, localMsg],
          },
          messageGroups: groupMessagesByDate([...existing, localMsg]),
        };
      });
    } catch (e) {
      throw new Error(e instanceof Error ? e.message : "发送消息失败");
    }
  },

  clearSearch: () => {
    set({ searchQuery: "", searchResults: [], isSearching: false });
  },

  pollNewMessages: async (room) => {
    const targetRoom = room ?? get().selectedRoom;
    if (!targetRoom) return;

    const key = roomKey(targetRoom);
    const existing = get().messagesByRoom[key] ?? [];
    if (existing.length === 0) return;

    const maxBackendId = Math.max(...existing.map((m) => m.backendId));
    if (maxBackendId <= 0) return;

    try {
      const page = await fetchChatMessages({
        displayName: targetRoom.displayName,
        cursorId: maxBackendId,
        direction: "newer",
        pageSize: 100,
      });

      if (page.messages.length === 0) return;

      set((state) => {
        const merged = uniqueSortedMessages([...existing, ...page.messages]);
        const nextState = {
          ...state,
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
        };
        return {
          ...nextState,
          ...buildCurrentDerivedState(nextState),
        };
      });
    } catch {
      // 轮询静默失败，下次继续
    }
  },
}));

export default useChatStore;
