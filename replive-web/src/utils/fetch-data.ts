import Axios from "axios";
import {
  buildWebStorage,
  type CacheRequestConfig,
  setupCache,
} from "axios-cache-interceptor";
import type {
  BackendChatRoom,
  BackendMessage,
  ChatMessagesPage,
  ChatRoom,
  Message,
  UserProfile,
} from "../types/chat";

interface ApiResponse<T> {
  success?: boolean;
  code?: number;
  msg?: string;
  data: T;
}

interface BackendMessagesPage {
  messages: BackendMessage[];
  next_cursor_id?: number;
  prev_cursor_id?: number;
  has_more?: boolean;
  has_older?: boolean;
  has_newer?: boolean;
  anchor_id?: number;
}

interface FetchChatMessagesParams {
  displayName: string;
  cursorId?: number;
  anchorId?: number;
  date?: string;
  direction?: "older" | "newer" | "around";
  pageSize?: number;
}

interface SearchChatMessagesParams {
  displayName: string;
  keyword: string;
  cursorId?: number;
  pageSize?: number;
}

interface TranslateTextParams {
  text: string;
  source?: string;
  target?: string;
}

interface TranslateData {
  translated_text: string;
}

const PAGE_SIZE_FALLBACK = 30;
const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? "";

const axios = setupCache(
  Axios.create({
    baseURL: apiBaseUrl,
  }),
  {
    storage: buildWebStorage(localStorage, "axios-cache:"),
  },
);

function unwrapResponse<T>(response: ApiResponse<T>): T {
  if (response.success === false || response.code === -1) {
    throw new Error(response.msg || "接口请求失败");
  }

  return response.data;
}

function mapChatRoom(room: BackendChatRoom): ChatRoom {
  return {
    id: room.id,
    userId: room.user_id,
    chatRoomId: room.chat_room_id,
    displayName: room.display_name,
    avatarUrl: room.avatar_url,
  };
}

function mapMessage(message: BackendMessage): Message {
  const type = mapMessageType(message.msg_type);
  const mediaUrl =
    type === "image"
      ? message.image_url
      : type === "video"
        ? message.video_url
        : undefined;

  return {
    id: message.chat_message_id || String(message.id),
    backendId: message.id,
    chatMessageId: message.chat_message_id || String(message.id),
    content: message.content || mediaPlaceholder(type),
    type,
    createdAt: mapCreatedAt(message),
    mediaUrl: mediaUrl || undefined,
    senderId: message.user_id,        // 发送者ID
    senderName: message.display_name, // 发送者名字
  };
}

function mapMessageType(msgType: number): Message["type"] {
  if (msgType === 2) return "image";
  if (msgType === 3) return "video";
  return "text";
}

function mediaPlaceholder(type: Message["type"]) {
  if (type === "image") return "[图片]";
  if (type === "video") return "[视频]";
  return "";
}

function mapCreatedAt(message: BackendMessage) {
  if (message.send_time && message.send_time > 0) {
    const millis =
      message.send_time > 10_000_000_000
        ? message.send_time
        : message.send_time * 1000;
    return new Date(millis).toISOString();
  }

  if (message.time_str) {
    const parsed = new Date(message.time_str);
    if (!Number.isNaN(parsed.getTime())) return parsed.toISOString();
  }

  return new Date().toISOString();
}

function sortMessagesAsc(messages: Message[]) {
  return messages.sort((a, b) => {
    if (a.backendId !== b.backendId) return a.backendId - b.backendId;
    return new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime();
  });
}

export async function fetchChatRooms(): Promise<ChatRoom[]> {
  const response = await axios.get<ApiResponse<BackendChatRoom[]>>(
    "/api/chat/rooms",
  );
  return unwrapResponse(response.data).map(mapChatRoom);
}

export async function fetchChatMessages({
  displayName,
  cursorId = 0,
  anchorId = 0,
  date,
  direction,
  pageSize = PAGE_SIZE_FALLBACK,
}: FetchChatMessagesParams): Promise<ChatMessagesPage> {
  const response = await axios.get<ApiResponse<BackendMessagesPage>>(
    "/api/chat/messages",
    {
      params: {
        display_name: displayName,
        page_size: pageSize,
        ...(cursorId > 0 ? { cursor_id: cursorId } : {}),
        ...(anchorId > 0 ? { anchor_id: anchorId } : {}),
        ...(date ? { date } : {}),
        ...(direction ? { direction } : {}),
      },
    } satisfies CacheRequestConfig,
  );
  const data = unwrapResponse(response.data);
  const messages = sortMessagesAsc(data.messages.map(mapMessage));
  const nextCursorId = data.next_cursor_id ?? 0;
  const prevCursorId = data.prev_cursor_id ?? 0;
  const hasMore =
    data.has_more ?? (nextCursorId > 0 && data.messages.length === pageSize);

  return {
    messages,
    nextCursorId,
    prevCursorId,
    hasMore,
    hasOlder: data.has_older ?? hasMore,
    hasNewer: data.has_newer ?? false,
    anchorId: data.anchor_id ?? 0,
  };
}

export async function searchChatMessages({
  displayName,
  keyword,
  cursorId = 0,
  pageSize = 30,
}: SearchChatMessagesParams): Promise<ChatMessagesPage> {
  const response = await axios.get<ApiResponse<BackendMessagesPage>>(
    "/api/chat/search",
    {
      params: {
        display_name: displayName,
        keyword,
        page_size: pageSize,
        ...(cursorId > 0 ? { cursor_id: cursorId } : {}),
      },
    } satisfies CacheRequestConfig,
  );
  const data = unwrapResponse(response.data);
  const messages = sortMessagesAsc(data.messages.map(mapMessage));
  const nextCursorId = data.next_cursor_id ?? 0;

  return {
    messages,
    nextCursorId,
    prevCursorId: data.prev_cursor_id ?? 0,
    hasMore: data.has_more ?? false,
    hasOlder: data.has_older ?? false,
    hasNewer: data.has_newer ?? false,
    anchorId: data.anchor_id ?? 0,
  };
}

export interface SendMessageParams {
  chatRoomId: string;
  content: string;
}

export async function sendChatMessage({
  chatRoomId,
  content,
}: SendMessageParams): Promise<void> {
  const response = await axios.post<ApiResponse<unknown>>("/api/chat/send", {
    chat_room_id: chatRoomId,
    content,
  });
  unwrapResponse(response.data);
}

export async function fetchUserProfile(): Promise<UserProfile> {
  const response = await axios.get<ApiResponse<BackendUserProfile>>(
    "/api/user/me",
    { cache: false } satisfies CacheRequestConfig,
  );
  const data = unwrapResponse(response.data);
  return {
    userId: data.user_id,
    uniqueId: data.unique_id,
    displayName: data.display_name,
    avatarUrl: data.avatar_url,
    sendChatEnabled: data.send_chat,
  };
}

export async function translateText({
  text,
  source = "ja",
  target = "zh-CN",
}: TranslateTextParams): Promise<string> {
  const trimmedText = text.trim();
  if (!trimmedText) return "";

  const response = await axios.get<ApiResponse<TranslateData>>(
    "/api/translate",
    {
      cache: false,
      params: {
        text: trimmedText,
        source,
        target,
      },
    } satisfies CacheRequestConfig,
  );

  return unwrapResponse(response.data).translated_text;
}
