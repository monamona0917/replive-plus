import { create } from "zustand";
import { formatDateKey } from "../lib/utils";
import type {
  ChatCategory,
  ChatRoom,
  JumpTarget,
  MediaItem,
  Message,
  MessageGroup,
  UserProfile,
} from "../types/chat";
import {
  fetchChatMessages,
  fetchChatRooms,
  fetchRoomAvailableDates,
  fetchRoomMediaPage,
  fetchUserProfile,
  searchChatMessages,
  sendChatMessage,
} from "../utils/fetch-data";
import type { MediaPage } from "../utils/fetch-data";

const PAGE_SIZE = 30;
const JUMP_FILL_PAGE_SIZE = 100;
const INITIAL_MEDIA_PAGE_SIZE = 1000;
const BACKGROUND_MEDIA_PAGE_SIZE = 300;
const MEDIA_PAGE_YIELD_MS = 50;
let jumpRequestIdCounter = 0;

type MediaType = "image" | "video";

interface RoomMediaLoadState {
  imageCursor: number;
  imageHasOlder: boolean;
  imageLoaded: boolean;
  videoCursor: number;
  videoHasOlder: boolean;
  videoLoaded: boolean;
  initialLoaded: boolean;
  isLoading: boolean;
  isComplete: boolean;
}

const mediaAbortControllers = new Map<string, AbortController>();

function emptyRoomMediaLoadState(): RoomMediaLoadState {
  return {
    imageCursor: 0,
    imageHasOlder: false,
    imageLoaded: false,
    videoCursor: 0,
    videoHasOlder: false,
    videoLoaded: false,
    initialLoaded: false,
    isLoading: false,
    isComplete: false,
  };
}

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
  mediaList: MediaItem[];
  mediaListByRoom: Record<string, MediaItem[]>;
  mediaLoadStateByRoom: Record<string, RoomMediaLoadState>;
  isLoadingMedia: boolean;

  // 可用日期列表（用于日历高亮）
  availableDatesByRoom: Record<string, string[]>;
  isLoadingDates: boolean;

  // 搜索与版本化跳转
  searchQuery: string;
  searchResults: Message[];
  isSearching: boolean;
  jumpTarget: JumpTarget | null;
  roomEpoch: Record<string, number>;
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
  clearJumpTarget: (requestId?: number) => void;
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
      dateStr = formatDateKey(message.createdAt);
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
        fallbackUrl: msg.mediaFallbackUrl,
        createdAt: msg.createdAt,
        senderName: msg.senderName,
        messageId: msg.id,
        backendId: msg.backendId,
      });
    }
  }
  return items;
}

function mergeMediaItems(...groups: MediaItem[][]): MediaItem[] {
  const byID = new Map<string, MediaItem>();
  for (const group of groups) {
    for (const item of group) {
      byID.set(item.id, item);
    }
  }
  return Array.from(byID.values()).sort((a, b) => {
    const timeDiff = new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime();
    if (timeDiff !== 0) return timeDiff;
    return b.backendId - a.backendId;
  });
}

function extractDatesFromMessages(messages: Message[]): string[] {
  const dates = new Set<string>();
  for (const msg of messages) {
    try {
      // 日期列表与后端都按日本时区计算，不能使用浏览器本地时区。
      const dateKey = formatDateKey(msg.createdAt);
      if (/^\d{4}-\d{2}-\d{2}$/.test(dateKey)) dates.add(dateKey);
    } catch {
      // 忽略无法解析的格式
    }
  }
  return Array.from(dates).sort();
}

function uniqueSortedMessages(messages: Message[]): Message[] {
  const byId = new Map<string, Message>();
  for (const msg of messages) {
    const existing = byId.get(msg.id);
    byId.set(
      msg.id,
      existing
        ? {
            ...existing,
            ...msg,
            mediaUrl: msg.mediaUrl || existing.mediaUrl,
            mediaFallbackUrl:
              msg.mediaFallbackUrl || existing.mediaFallbackUrl,
          }
        : msg,
    );
  }
  return Array.from(byId.values()).sort((a, b) => {
    const timeDiff =
      new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime();
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
  mediaList: [],
  mediaListByRoom: {},
  mediaLoadStateByRoom: {},
  isLoadingMedia: false,

  availableDatesByRoom: {},
  isLoadingDates: false,

  searchQuery: "",
  searchResults: [],
  isSearching: false,
  jumpTarget: null,
  roomEpoch: {},
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

  setActiveCategory: (category) => {
    set({ activeCategory: category });
    const currentRoom = get().selectedRoom;
    if (currentRoom && (currentRoom.category || "fandom") !== category) {
      const rooms = get().rooms;
      const firstInCat = rooms.find(
        (r) => (r.category || "fandom") === category,
      );
      if (firstInCat) {
        void get().selectRoom(firstInCat);
      }
    }
  },

  loadUserProfile: async () => {
    try {
      const profile = await fetchUserProfile();
      set({ userProfile: profile });
    } catch {
      // 静默失败
    }
  },

  loadRooms: async () => {
    set({ isLoadingRooms: true, error: null });
    try {
      const rooms = await fetchChatRooms();
      const currentRoom = get().selectedRoom;
      const updatedSelected = currentRoom
        ? rooms.find((r) => roomKey(r) === roomKey(currentRoom)) || currentRoom
        : null;

      set({
        rooms,
        selectedRoom: updatedSelected,
        isLoadingRooms: false,
      });
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
    const previousRoom = get().selectedRoom;
    const previousKey = previousRoom ? roomKey(previousRoom) : "";

    if (previousKey && previousKey !== key) {
      mediaAbortControllers.get(previousKey)?.abort();
      mediaAbortControllers.delete(previousKey);
      set((state) => {
        const previousMediaState = state.mediaLoadStateByRoom[previousKey];
        if (!previousMediaState?.isLoading) return state;
        return {
          mediaLoadStateByRoom: {
            ...state.mediaLoadStateByRoom,
            [previousKey]: {
              ...previousMediaState,
              isLoading: false,
            },
          },
        };
      });
    }

    // 递增该房间代次并重置在途跳转状态
    jumpRequestIdCounter++;
    const currentEpoch = get().roomEpoch[key] ?? 0;
    const myEpoch = currentEpoch + 1;

    set((state) => {
      const messages = state.messagesByRoom[key] ?? [];
      const currentMedia = mergeMediaItems(
        state.mediaListByRoom[key] ?? [],
        extractMediaItems(messages),
      );
      return {
        selectedRoom: room,
        activeCategory: roomCat,
        searchQuery: "",
        searchResults: [],
        mediaList: currentMedia,
        isLoadingMedia: state.mediaLoadStateByRoom[key]?.isLoading ?? false,
        error: null,
        jumpTarget: null,
        isLoadingMessagesByRoom: {
          ...state.isLoadingMessagesByRoom,
          [key]: false,
        },
        isLoadingMore: false,
        isLoadingNewer: false,
        roomEpoch: {
          ...state.roomEpoch,
          [key]: myEpoch,
        },
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

    // 先读取最新媒体，再在后台按游标补齐更早记录。
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
    const existingState = get().mediaLoadStateByRoom[key] ?? emptyRoomMediaLoadState();
    if (existingState.isLoading || existingState.isComplete) return;

    const controller = new AbortController();
    mediaAbortControllers.get(key)?.abort();
    mediaAbortControllers.set(key, controller);

    const isCurrentRoom = (state: ChatState) =>
      state.selectedRoom !== null && roomKey(state.selectedRoom) === key;
    const updateLoadState = (
      update: (current: RoomMediaLoadState) => RoomMediaLoadState,
    ) => {
      set((state) => {
        const next = update(
          state.mediaLoadStateByRoom[key] ?? emptyRoomMediaLoadState(),
        );
        return {
          mediaLoadStateByRoom: {
            ...state.mediaLoadStateByRoom,
            [key]: next,
          },
          ...(isCurrentRoom(state) ? { isLoadingMedia: next.isLoading } : {}),
        };
      });
    };
    const mergeMediaPage = (items: MediaItem[]) => {
      if (items.length === 0) return;
      set((state) => {
        const merged = mergeMediaItems(
          state.mediaListByRoom[key] ?? [],
          extractMediaItems(state.messagesByRoom[key] ?? []),
          items,
        );
        return {
          mediaListByRoom: {
            ...state.mediaListByRoom,
            [key]: merged,
          },
          ...(isCurrentRoom(state) ? { mediaList: merged } : {}),
        };
      });
    };
    const applyPage = (
      mediaType: MediaType,
      page: MediaPage,
      initial: boolean,
      seenCursors: Set<number>,
    ) => {
      mergeMediaPage(page.items);
      updateLoadState((current) => {
        const oldCursor = mediaType === "image" ? current.imageCursor : current.videoCursor;
        const canContinue = !page.hasOlder || (
          page.items.length > 0 &&
          page.olderCursorId > 0 &&
          page.olderCursorId !== oldCursor &&
          !seenCursors.has(page.olderCursorId)
        );
        if (!initial && oldCursor > 0) {
          seenCursors.add(oldCursor);
        }
        const hasOlder = page.hasOlder && canContinue;
        const next = mediaType === "image"
          ? {
              ...current,
              imageCursor: page.olderCursorId,
              imageHasOlder: hasOlder,
              imageLoaded: true,
            }
          : {
              ...current,
              videoCursor: page.olderCursorId,
              videoHasOlder: hasOlder,
              videoLoaded: true,
            };
        next.initialLoaded = next.imageLoaded && next.videoLoaded;
        next.isComplete = next.initialLoaded && !next.imageHasOlder && !next.videoHasOlder;
        return next;
      });
    };
    const fetchPage = (mediaType: MediaType, cursorId: number, pageSize: number) =>
      fetchRoomMediaPage({
        room: targetRoom,
        mediaType,
        cursorId,
        pageSize,
        signal: controller.signal,
      });

    updateLoadState((current) => ({ ...current, isLoading: true }));
    const seenCursors = {
      image: new Set<number>(),
      video: new Set<number>(),
    };

    try {
      const initialState = get().mediaLoadStateByRoom[key] ?? emptyRoomMediaLoadState();
      const initialTypes = (["image", "video"] as MediaType[]).filter(
        (mediaType) =>
          mediaType === "image" ? !initialState.imageLoaded : !initialState.videoLoaded,
      );

      if (initialTypes.length > 0) {
        const initialResults = await Promise.allSettled(
          initialTypes.map((mediaType) =>
            fetchPage(mediaType, 0, INITIAL_MEDIA_PAGE_SIZE),
          ),
        );
        if (controller.signal.aborted) return;

        let initialFailed = false;
        for (let index = 0; index < initialResults.length; index++) {
          const result = initialResults[index];
          const mediaType = initialTypes[index];
          if (result.status !== "fulfilled") {
            initialFailed = true;
            continue;
          }
          applyPage(mediaType, result.value, true, seenCursors[mediaType]);
        }
        if (initialFailed || !(get().mediaLoadStateByRoom[key]?.initialLoaded)) {
          updateLoadState((current) => ({ ...current, isLoading: false }));
          return;
        }
      }

      while (!controller.signal.aborted) {
        const current = get().mediaLoadStateByRoom[key] ?? emptyRoomMediaLoadState();
        if (!current.imageHasOlder && !current.videoHasOlder) {
          updateLoadState((state) => ({
            ...state,
            isLoading: false,
            isComplete: state.initialLoaded,
          }));
          return;
        }

        await new Promise((resolve) => window.setTimeout(resolve, MEDIA_PAGE_YIELD_MS));
        if (controller.signal.aborted) return;

        const pageTypes = (["image", "video"] as MediaType[]).filter(
          (mediaType) =>
            mediaType === "image" ? current.imageHasOlder : current.videoHasOlder,
        );
        const results = await Promise.allSettled(
          pageTypes.map((mediaType) =>
            fetchPage(
              mediaType,
              mediaType === "image" ? current.imageCursor : current.videoCursor,
              BACKGROUND_MEDIA_PAGE_SIZE,
            ),
          ),
        );
        if (controller.signal.aborted) return;

        let pageFailed = false;
        for (let index = 0; index < results.length; index++) {
          const result = results[index];
          const mediaType = pageTypes[index];
          if (result.status !== "fulfilled") {
            pageFailed = true;
            continue;
          }
          applyPage(mediaType, result.value, false, seenCursors[mediaType]);
        }
        if (pageFailed) {
          updateLoadState((state) => ({ ...state, isLoading: false }));
          return;
        }
      }
    } finally {
      if (mediaAbortControllers.get(key) === controller) {
        mediaAbortControllers.delete(key);
        updateLoadState((current) =>
          current.isLoading ? { ...current, isLoading: false } : current,
        );
      }
    }
  },

  loadLatestMessages: async (room?: ChatRoom) => {
    const targetRoom = room ?? get().selectedRoom;
    if (!targetRoom) return;

    const key = roomKey(targetRoom);
    if (get().isLoadingMessagesByRoom[key]) return;
    if (get().jumpTarget?.roomKey === key) return;

    const myEpoch = get().roomEpoch[key] ?? 0;

    set((state) => ({
      isLoadingMessagesByRoom: {
        ...state.isLoadingMessagesByRoom,
        [key]: true,
      },
      isLoadingMore: false,
      isLoadingNewer: false,
      error: null,
    }));

    try {
      const page = await fetchChatMessages({
        room: targetRoom,
        pageSize: PAGE_SIZE,
      });

      // 校验代次：若期间发生了跳转或切房，直接丢弃，不能改动新请求的状态
      if ((get().roomEpoch[key] ?? 0) !== myEpoch) {
        return;
      }

      const pageMessages = uniqueSortedMessages(page.messages);

      set((state) => {
        if ((state.roomEpoch[key] ?? 0) !== myEpoch) {
          return state;
        }

        const pendingMessages = (state.messagesByRoom[key] ?? []).filter(
          (message) => message.backendId === 0,
        );
        // 服务端记录排在后面，以正式记录替换同 ID 的本地待同步消息。
        const sorted = uniqueSortedMessages([...pendingMessages, ...pageMessages]);
        const localMedia = extractMediaItems(sorted);
        const localDates = extractDatesFromMessages(sorted);
        const existingMedia = state.mediaListByRoom[key] || [];
        const mergedMedia = mergeMediaItems(existingMedia, localMedia);

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
        };
      });
    } catch (err) {
      if ((get().roomEpoch[key] ?? 0) !== myEpoch) return;
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

    const myEpoch = get().roomEpoch[key] ?? 0;
    set({ isLoadingMore: true, error: null });

    try {
      const page = await fetchChatMessages({ room: targetRoom, cursorId, pageSize: PAGE_SIZE });

      if ((get().roomEpoch[key] ?? 0) !== myEpoch) {
        return;
      }

      set((state) => {
        if ((state.roomEpoch[key] ?? 0) !== myEpoch) {
          return state;
        }
        const existing = state.messagesByRoom[key] ?? [];
        const merged = uniqueSortedMessages([...page.messages, ...existing]);
        const existingMedia = state.mediaListByRoom[key] ?? [];
        const mergedMedia = mergeMediaItems(
          existingMedia,
          extractMediaItems(page.messages),
        );
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
          mediaListByRoom: {
            ...state.mediaListByRoom,
            [key]: mergedMedia,
          },
          ...(state.selectedRoom && roomKey(state.selectedRoom) === key
            ? { mediaList: mergedMedia }
            : {}),
          availableDatesByRoom: {
            ...state.availableDatesByRoom,
            [key]: mergedDates,
          },
          isLoadingMore: false,
        };
      });
    } catch (err) {
      if ((get().roomEpoch[key] ?? 0) !== myEpoch) return;
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

    const myEpoch = get().roomEpoch[key] ?? 0;
    set({ isLoadingNewer: true, error: null });

    try {
      const page = await fetchChatMessages({
        room: targetRoom,
        cursorId,
        direction: "newer",
        pageSize: PAGE_SIZE,
      });

      if ((get().roomEpoch[key] ?? 0) !== myEpoch) {
        return;
      }

      set((state) => {
        if ((state.roomEpoch[key] ?? 0) !== myEpoch) {
          return state;
        }
        const existing = state.messagesByRoom[key] ?? [];
        const merged = uniqueSortedMessages([...existing, ...page.messages]);
        const existingMedia = state.mediaListByRoom[key] ?? [];
        const mergedMedia = mergeMediaItems(
          existingMedia,
          extractMediaItems(page.messages),
        );
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
          mediaListByRoom: {
            ...state.mediaListByRoom,
            [key]: mergedMedia,
          },
          ...(state.selectedRoom && roomKey(state.selectedRoom) === key
            ? { mediaList: mergedMedia }
            : {}),
          availableDatesByRoom: {
            ...state.availableDatesByRoom,
            [key]: mergedDates,
          },
          isLoadingNewer: false,
        };
      });
    } catch (err) {
      if ((get().roomEpoch[key] ?? 0) !== myEpoch) return;
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
    const nextRequestId = ++jumpRequestIdCounter;
    const currentEpoch = get().roomEpoch[key] ?? 0;
    const myEpoch = currentEpoch + 1;

    // 发起时立即置位跳转锁与代次
    set((state) => ({
      roomEpoch: {
        ...state.roomEpoch,
        [key]: myEpoch,
      },
      jumpTarget: {
        roomKey: key,
        messageId: null,
        date,
        requestId: nextRequestId,
      },
      isLoadingMessagesByRoom: {
        ...state.isLoadingMessagesByRoom,
        [key]: true,
      },
      isLoadingMore: false,
      isLoadingNewer: false,
      error: null,
      dateJumpModalOpen: false,
    }));

    // 最新窗口通常已经包含目标日期。优先使用当前内存中的权威消息，
    // 避免为一个已加载日期重新请求 around 页面并把滚动容器短暂清空。
    const localTarget = (get().messagesByRoom[key] ?? []).find(
      (message) => formatDateKey(message.createdAt) === date,
    );
    if (localTarget) {
      set((state) => ({
        isLoadingMessagesByRoom: {
          ...state.isLoadingMessagesByRoom,
          [key]: false,
        },
        jumpTarget: {
          roomKey: key,
          messageId: localTarget.id,
          backendId: localTarget.backendId,
          date,
          requestId: nextRequestId,
        },
      }));
      return;
    }

    try {
      let page = await fetchChatMessages({
        room: targetRoom,
        date,
        direction: "around",
        pageSize: PAGE_SIZE,
      });

      // 竞态校验：代次或 requestId 不匹配则直接丢弃
      if (
        (get().roomEpoch[key] ?? 0) !== myEpoch ||
        get().jumpTarget?.requestId !== nextRequestId
      ) {
        return;
      }

      const findDateTarget = (messages: Message[], anchorId = 0) => {
        const anchorBackendId = Number(anchorId);
        return (
          (anchorBackendId > 0
            ? messages.find((m) => Number(m.backendId) === anchorBackendId)
            : undefined) ??
          messages.find((m) => formatDateKey(m.createdAt) === date)
        );
      };

      let targetMsg = findDateTarget(page.messages, page.anchorId);

      // 如果后端的 date/anchor 查询没有命中，沿用相册和搜索跳转使用的
      // “已知游标继续向前取”的方式，自动加载更早页面直到找到目标日期。
      // 用户不需要先手动拖到顶部触发历史分页。
      if (!targetMsg) {
        let merged = uniqueSortedMessages([
          ...(get().messagesByRoom[key] ?? []),
          ...page.messages,
        ]);
        let cursorId = get().cursorByRoom[key] ?? 0;
        if (cursorId <= 0) {
          cursorId = merged.reduce((oldest, message) => {
            if (message.backendId <= 0) return oldest;
            return oldest <= 0 || message.backendId < oldest
              ? message.backendId
              : oldest;
          }, 0);
        }
        let hasOlder = get().hasMoreByRoom[key] ?? cursorId > 0;
        const seenOlderCursors = new Set<number>();

        while (!targetMsg && hasOlder && cursorId > 0) {
          if (seenOlderCursors.has(cursorId)) {
            hasOlder = false;
            break;
          }
          seenOlderCursors.add(cursorId);

          const olderPage = await fetchChatMessages({
            room: targetRoom,
            cursorId,
            pageSize: JUMP_FILL_PAGE_SIZE,
          });
          if (
            (get().roomEpoch[key] ?? 0) !== myEpoch ||
            get().jumpTarget?.requestId !== nextRequestId
          ) {
            return;
          }

          page = olderPage;
          merged = uniqueSortedMessages([...olderPage.messages, ...merged]);
          targetMsg = findDateTarget(merged);
          const nextCursorId = olderPage.prevCursorId || olderPage.nextCursorId;
          const cursorProgressed =
            olderPage.messages.length > 0 &&
            nextCursorId > 0 &&
            nextCursorId !== cursorId &&
            !seenOlderCursors.has(nextCursorId);
          cursorId = nextCursorId;
          hasOlder = olderPage.hasOlder && cursorProgressed;

          set((state) => ({
            messagesByRoom: {
              ...state.messagesByRoom,
              [key]: merged,
            },
            cursorByRoom: {
              ...state.cursorByRoom,
              [key]: cursorId,
            },
            hasMoreByRoom: {
              ...state.hasMoreByRoom,
              [key]: hasOlder,
            },
          }));

          if (!cursorProgressed) break;
        }

        if (targetMsg) {
          const resolvedTargetMsg = targetMsg;
          set((state) => ({
            isLoadingMessagesByRoom: {
              ...state.isLoadingMessagesByRoom,
              [key]: false,
            },
            hasNewerByRoom: {
              ...state.hasNewerByRoom,
              [key]: true,
            },
            jumpTarget: {
              roomKey: key,
              messageId: resolvedTargetMsg.id,
              backendId: resolvedTargetMsg.backendId,
              date,
              requestId: nextRequestId,
            },
          }));
          return;
        }
      }

      if (!targetMsg) {
        set((state) => ({
          error: page.messages.length === 0 ? "该日期暂无聊天记录" : "未能定位到该日期的目标消息",
          isLoadingMessagesByRoom: {
            ...state.isLoadingMessagesByRoom,
            [key]: false,
          },
          jumpTarget: null,
        }));
        return;
      }

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
        jumpTarget: {
          roomKey: key,
          messageId: targetMsg.id,
          backendId: targetMsg.backendId,
          date,
          requestId: nextRequestId,
        },
        dateJumpModalOpen: false,
      }));
    } catch (err) {
      if (
        (get().roomEpoch[key] ?? 0) !== myEpoch ||
        get().jumpTarget?.requestId !== nextRequestId
      ) {
        return;
      }
      set((state) => ({
        error: err instanceof Error ? err.message : "加载该日期消息失败",
        isLoadingMessagesByRoom: {
          ...state.isLoadingMessagesByRoom,
          [key]: false,
        },
        jumpTarget: null,
      }));
    }
  },

  jumpToMessage: async (message: Message, room?: ChatRoom) => {
    const targetRoom = room ?? get().selectedRoom;
    if (!targetRoom) return;

    const key = roomKey(targetRoom);
    const nextRequestId = ++jumpRequestIdCounter;
    const currentEpoch = get().roomEpoch[key] ?? 0;
    const myEpoch = currentEpoch + 1;

    // 发起时立即置位跳转锁与代次
    set((state) => ({
      roomEpoch: {
        ...state.roomEpoch,
        [key]: myEpoch,
      },
      jumpTarget: {
        roomKey: key,
        messageId: null,
        backendId: message.backendId,
        requestId: nextRequestId,
      },
      isLoadingMessagesByRoom: {
        ...state.isLoadingMessagesByRoom,
        [key]: true,
      },
      isLoadingMore: false,
      isLoadingNewer: false,
      error: null,
      searchDrawerOpen: false,
      mediaGalleryDrawerOpen: false,
    }));

    try {
      let page = await fetchChatMessages({
        room: targetRoom,
        anchorId: message.backendId,
        direction: "around",
        pageSize: PAGE_SIZE,
      });

      // 竞态校验：代次或 requestId 不匹配则直接丢弃
      if (
        (get().roomEpoch[key] ?? 0) !== myEpoch ||
        get().jumpTarget?.requestId !== nextRequestId
      ) {
        return;
      }

      // 通过实际返回的权威消息数组匹配 backendId 或 id，绝不静默回退
      const requestedBackendId = Number(message.backendId);
      const targetMsg =
        requestedBackendId > 0
          ? page.messages.find((m) => Number(m.backendId) === requestedBackendId) ?? message
          : page.messages.find((m) => m.id === message.id) ?? message;

      if (!targetMsg) {
        set((state) => ({
          error: "未能定位到目标消息",
          isLoadingMessagesByRoom: {
            ...state.isLoadingMessagesByRoom,
            [key]: false,
          },
          jumpTarget: null,
        }));
        return;
      }

      // 相册/搜索跳转也补齐目标之后的所有消息。around 只返回目标附近的一页，
      // 如果直接提交这页，滚动条底部只是局部窗口而不是最新消息。
      let messagesForJump = uniqueSortedMessages(
        page.messages.some((m) => m.id === targetMsg.id)
          ? page.messages
          : [...page.messages, targetMsg],
      );
      const olderCursor = page.prevCursorId;
      const hasOlder = page.hasOlder;
      let hasNewer = page.hasNewer;
      let newerCursor = page.nextCursorId;
      const seenNewerCursors = new Set<number>();
      while (hasNewer && newerCursor > 0) {
        if (seenNewerCursors.has(newerCursor)) break;
        seenNewerCursors.add(newerCursor);

        const newerPage = await fetchChatMessages({
          room: targetRoom,
          cursorId: newerCursor,
          direction: "newer",
          pageSize: JUMP_FILL_PAGE_SIZE,
        });
        if (
          (get().roomEpoch[key] ?? 0) !== myEpoch ||
          get().jumpTarget?.requestId !== nextRequestId
        ) {
          return;
        }

        page = newerPage;
        messagesForJump = uniqueSortedMessages([
          ...messagesForJump,
          ...newerPage.messages,
        ]);
        hasNewer = newerPage.hasNewer;
        const nextCursorId = newerPage.nextCursorId;
        const cursorProgressed =
          newerPage.messages.length > 0 &&
          nextCursorId > 0 &&
          nextCursorId !== newerCursor &&
          !seenNewerCursors.has(nextCursorId);
        newerCursor = nextCursorId;
        if (!cursorProgressed) break;
      }

      const sorted = messagesForJump;

      set((state) => ({
        messagesByRoom: {
          ...state.messagesByRoom,
          [key]: sorted,
        },
        cursorByRoom: {
          ...state.cursorByRoom,
          [key]: olderCursor,
        },
        newerCursorByRoom: {
          ...state.newerCursorByRoom,
          [key]: page.nextCursorId,
        },
        hasMoreByRoom: {
          ...state.hasMoreByRoom,
          [key]: hasOlder,
        },
        hasNewerByRoom: {
          ...state.hasNewerByRoom,
          [key]: hasNewer,
        },
        isLoadingMessagesByRoom: {
          ...state.isLoadingMessagesByRoom,
          [key]: false,
        },
        jumpTarget: {
          roomKey: key,
          messageId: targetMsg.id,
          backendId: targetMsg.backendId,
          requestId: nextRequestId,
        },
        searchDrawerOpen: false,
        mediaGalleryDrawerOpen: false,
      }));
    } catch (err) {
      if (
        (get().roomEpoch[key] ?? 0) !== myEpoch ||
        get().jumpTarget?.requestId !== nextRequestId
      ) {
        return;
      }
      set((state) => ({
        error: err instanceof Error ? err.message : "跳转到目标消息失败",
        isLoadingMessagesByRoom: {
          ...state.isLoadingMessagesByRoom,
          [key]: false,
        },
        jumpTarget: null,
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
        senderId: "me",
        senderName: "me",
      };

      set((state) => {
        const existing = state.messagesByRoom[key] ?? [];
        const merged = uniqueSortedMessages([...existing, localMsg]);
        return {
          messagesByRoom: {
            ...state.messagesByRoom,
            [key]: merged,
          },
        };
      });

      // 滚动到底部
      set((state) => ({
        scrollToBottomToken: state.scrollToBottomToken + 1,
      }));
    } catch (err) {
      set({
        error: err instanceof Error ? err.message : "发送消息失败",
      });
      throw err;
    }
  },

  pollNewMessages: async (room?: ChatRoom) => {
    const targetRoom = room ?? get().selectedRoom;
    if (!targetRoom || targetRoom.category === "prime") return;

    const key = roomKey(targetRoom);
    // 若当前房间处于跳转中或 loading 中，跳过轮询
    if (get().jumpTarget && get().jumpTarget?.roomKey === key) return;
    if (get().isLoadingMessagesByRoom[key]) return;
    if (get().hasNewerByRoom[key]) return;

    const myEpoch = get().roomEpoch[key] ?? 0;
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

      if ((get().roomEpoch[key] ?? 0) !== myEpoch) return;
      if (get().jumpTarget && get().jumpTarget?.roomKey === key) return;
      if (page.messages.length === 0) return;

      set((state) => {
        if ((state.roomEpoch[key] ?? 0) !== myEpoch) return state;
        if (state.jumpTarget && state.jumpTarget?.roomKey === key) return state;

        const currentMessages = state.messagesByRoom[key] ?? [];
        const merged = uniqueSortedMessages([...currentMessages, ...page.messages]);
        const existingMedia = state.mediaListByRoom[key] ?? [];
        const mergedMedia = mergeMediaItems(
          existingMedia,
          extractMediaItems(page.messages),
        );
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
          mediaListByRoom: {
            ...state.mediaListByRoom,
            [key]: mergedMedia,
          },
          ...(state.selectedRoom && roomKey(state.selectedRoom) === key
            ? { mediaList: mergedMedia }
            : {}),
        };
      });
    } catch {
      // 轮询静默失败
    }
  },

  clearJumpTarget: (requestId?: number) => {
    set((state) => {
      if (!requestId || state.jumpTarget?.requestId === requestId) {
        return { jumpTarget: null };
      }
      return state;
    });
  },

  requestScrollToBottom: async () => {
    const targetRoom = get().selectedRoom;
    if (!targetRoom) return;
    const key = roomKey(targetRoom);
    if (get().jumpTarget?.roomKey === key) return;
    if (get().hasNewerByRoom[key]) {
      await get().loadLatestMessages(targetRoom);
    }
    set((state) => ({
      scrollToBottomToken: state.scrollToBottomToken + 1,
    }));
  },

  setError: (error) => set({ error }),

  setSearchDrawerOpen: (open) => set({ searchDrawerOpen: open }),
  setMediaGalleryDrawerOpen: (open) => set({ mediaGalleryDrawerOpen: open }),
  setDateJumpModalOpen: (open) => set({ dateJumpModalOpen: open }),
  setAboutModalOpen: (open) => set({ aboutModalOpen: open }),

  openLightbox: (media) => set({ lightboxMedia: media }),
  closeLightbox: () => set({ lightboxMedia: null }),
  stepLightbox: (delta) => {
    const current = get().lightboxMedia;
    const list = get().mediaList;
    if (!current || list.length === 0) return;
    const idx = list.findIndex((m) => m.id === current.id);
    if (idx === -1) return;
    const nextIdx = (idx + delta + list.length) % list.length;
    set({ lightboxMedia: list[nextIdx] });
  },
}));

export default useChatStore;
