import Axios from "axios";
import type {
  BackendChatRoom,
  BackendMessage,
  BackendPrimeChatRoom,
  BackendPrimeMessage,
  BackendUserProfile,
  ChatMessagesPage,
  ChatRoom,
  MediaItem,
  Message,
  UserProfile,
} from "../types/chat";

interface ApiResponse<T> {
  success?: boolean;
  code?: number;
  msg?: string;
  data: T;
}

interface BackendMessagesPage<T> {
  messages: T[];
  next_cursor_id?: number;
  prev_cursor_id?: number;
  has_more?: boolean;
  has_older?: boolean;
  has_newer?: boolean;
  anchor_id?: number;
}

interface FetchChatMessagesParams {
  room: ChatRoom;
  cursorId?: number;
  anchorId?: number;
  date?: string;
  direction?: "older" | "newer" | "around";
  pageSize?: number;
  mediaType?: "image" | "video";
}

interface SearchChatMessagesParams {
  room: ChatRoom;
  keyword: string;
  cursorId?: number;
  pageSize?: number;
}

const PAGE_SIZE_FALLBACK = 30;
const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? "";

// Do not install browser-persistent response caching for chat data. Local
// SQLite is the source of truth and every UI request should see its latest row.
const axios = Axios.create({
  baseURL: apiBaseUrl,
  timeout: 8000,
  headers: {
    "Cache-Control": "no-store, no-cache, max-age=0",
    Pragma: "no-cache",
  },
});

function unwrapResponse<T>(response: ApiResponse<T>): T {
  if (response.success === false || response.code === -1) {
    throw new Error(response.msg || "API request failed");
  }
  return response.data;
}

function freshRequestParams(params: Record<string, string | number | undefined>) {
  return { ...params, _ts: Date.now() };
}

function normalizeEpochMilliseconds(value: unknown): number | undefined {
  if (typeof value === "number" && Number.isFinite(value) && value > 0) {
    return value < 100_000_000_000 ? value * 1000 : value;
  }

  if (typeof value === "string" && value.trim()) {
    const numeric = Number(value);
    if (Number.isFinite(numeric) && numeric > 0) {
      return numeric < 100_000_000_000 ? numeric * 1000 : numeric;
    }
    const parsed = Date.parse(value);
    if (Number.isFinite(parsed)) return parsed;
  }

  return undefined;
}

function mapFandomRoom(room: BackendChatRoom): ChatRoom {
  return {
    id: room.id,
    userId: room.user_id,
    uniqueId: room.unique_id || undefined,
    chatRoomId: room.chat_room_id,
    displayName: room.display_name,
    avatarUrl: room.avatar_url || undefined,
    talentLastCheckTime: normalizeEpochMilliseconds(room.talent_last_check_time),
    lastMessageTime: room.last_message_time
      ? new Date(room.last_message_time * 1000).toISOString()
      : undefined,
    lastMessageContent: room.last_message_content || undefined,
    lastMessageType: room.last_message_time
      ? mapMessageType(room.last_message_type ?? 0)
      : undefined,
    category: "fandom",
  };
}

function mapPrimeRoom(room: BackendPrimeChatRoom): ChatRoom {
  return {
    id: room.id,
    userId: room.talent_user_id,
    uniqueId: room.talent_unique_id || undefined,
    chatRoomId: room.chat_room_id,
    displayName: room.talent_display_name || room.talent_unique_id || room.talent_user_id,
    avatarUrl: room.talent_avatar_url || undefined,
    memberUserId: room.member_user_id || undefined,
    backgroundImageUrl: room.member_background_image_url || undefined,
    lastMessageTime: room.last_message_time
      ? new Date(room.last_message_time).toISOString()
      : undefined,
    lastMessageContent: room.last_message_content || undefined,
    lastMessageType: room.last_message_time
      ? mapPrimeMessageType(room.last_message_type)
      : undefined,
    category: "prime",
  };
}

function mapMessageType(msgType: number): Message["type"] {
  if (msgType === 2) return "image";
  if (msgType === 3) return "video";
  return "text";
}

function mapPrimeMessageType(bodyType?: string): Message["type"] {
  if (bodyType === "image") return "image";
  if (bodyType === "video") return "video";
  return "text";
}

function mediaPlaceholder(type: Message["type"]) {
  if (type === "image") return "[image]";
  if (type === "video") return "[video]";
  return "";
}

function mapFandomCreatedAt(message: BackendMessage) {
  if (message.send_time && message.send_time > 0) {
    const millis = message.send_time > 10_000_000_000 ? message.send_time : message.send_time * 1000;
    return new Date(millis).toISOString();
  }
  if (message.time_str) {
    const parsed = new Date(message.time_str);
    if (!Number.isNaN(parsed.getTime())) return parsed.toISOString();
  }
  return new Date(0).toISOString();
}

function mapPrimeCreatedAt(message: BackendPrimeMessage) {
  const millis = message.create_unix_time_ms ?? 0;
  if (millis > 0) return new Date(millis).toISOString();
  return new Date(0).toISOString();
}

function mapFandomMessage(message: BackendMessage): Message {
  const type = mapMessageType(message.msg_type);
  const remoteMediaUrl = type === "image" ? message.image_url : type === "video" ? message.video_url : undefined;
  const localMediaUrl = type === "image" ? message.image_local_url : type === "video" ? message.video_local_url : undefined;
  return {
    id: message.chat_message_id || String(message.id),
    backendId: message.id,
    chatMessageId: message.chat_message_id || String(message.id),
    content: message.content || mediaPlaceholder(type),
    type,
    createdAt: mapFandomCreatedAt(message),
    mediaUrl: localMediaUrl || remoteMediaUrl,
    mediaFallbackUrl: localMediaUrl && remoteMediaUrl ? remoteMediaUrl : undefined,
    senderId: message.user_id,
    senderName: message.display_name || "user",
  };
}

function mapPrimeMessage(message: BackendPrimeMessage, room: ChatRoom): Message {
  const rawType = mapPrimeMessageType(message.body_type);
  const senderKind = message.sender === "member" || message.sender === "talent" ? message.sender : undefined;
  const isDeleted = Boolean(message.is_deleted);
  const type = isDeleted ? "text" : rawType;
  const content = isDeleted
    ? "[message deleted]"
    : message.content || mediaPlaceholder(rawType);
  const isMember = senderKind === "member";

  return {
    id: message.message_id || String(message.id),
    backendId: message.id,
    chatMessageId: message.message_id || String(message.id),
    content,
    type,
    createdAt: mapPrimeCreatedAt(message),
    mediaUrl: rawType === "image" ? message.image_url : rawType === "video" ? message.video_url : undefined,
    mediaThumbnailUrl: rawType === "video" ? message.video_thumbnail_url || undefined : undefined,
    senderId: isMember ? message.member_user_id || room.memberUserId || "member" : message.chat_room_owner_user_id || room.userId,
    senderName: isMember ? "me" : room.displayName,
    senderKind,
    reactionEmoji: message.reaction_emoji || undefined,
    isDeleted,
    coinAmount: message.coin_amount,
  };
}

function sortMessagesAsc(messages: Message[]) {
  return messages.sort((a, b) => {
    const timeDiff = new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime();
    if (timeDiff !== 0) return timeDiff;
    return a.backendId - b.backendId;
  });
}

function endpointFor(room: ChatRoom, suffix: "messages" | "dates" | "search") {
  return room.category === "prime" ? `/api/prime/${suffix}` : `/api/chat/${suffix}`;
}

function roomParams(room: ChatRoom): Record<string, string> {
  return room.category === "prime"
    ? { chat_room_id: room.chatRoomId }
    : { display_name: room.displayName };
}

function mapPage<T>(data: BackendMessagesPage<T>, room: ChatRoom): ChatMessagesPage {
  const messages = sortMessagesAsc(
    data.messages.map((message) =>
      room.category === "prime"
        ? mapPrimeMessage(message as BackendPrimeMessage, room)
        : mapFandomMessage(message as BackendMessage),
    ),
  );
  const nextCursorId = data.next_cursor_id ?? 0;
  const prevCursorId = data.prev_cursor_id ?? 0;
  const hasMore = data.has_more ?? (nextCursorId > 0 && data.messages.length === PAGE_SIZE_FALLBACK);

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

export async function fetchChatRooms(): Promise<ChatRoom[]> {
  const [fandomResponse, primeResponse] = await Promise.all([
    axios.get<ApiResponse<BackendChatRoom[]>>("/api/chat/rooms", {
      params: freshRequestParams({}),
    }),
    axios.get<ApiResponse<BackendPrimeChatRoom[]>>("/api/prime/rooms", {
      params: freshRequestParams({}),
    }),
  ]);

  const fandomRooms = unwrapResponse(fandomResponse.data).map(mapFandomRoom);
  const primeRooms = unwrapResponse(primeResponse.data).map(mapPrimeRoom);
  return [...fandomRooms, ...primeRooms];
}

export async function fetchChatMessages({
  room,
  cursorId = 0,
  anchorId = 0,
  date,
  direction,
  pageSize = PAGE_SIZE_FALLBACK,
  mediaType,
}: FetchChatMessagesParams): Promise<ChatMessagesPage> {
  const response = await axios.get<ApiResponse<BackendMessagesPage<BackendMessage | BackendPrimeMessage>>>(
    endpointFor(room, "messages"),
    {
      params: freshRequestParams({
        ...roomParams(room),
        page_size: pageSize,
        ...(cursorId > 0 ? { cursor_id: cursorId } : {}),
        ...(anchorId > 0 ? { anchor_id: anchorId } : {}),
        ...(date ? { date } : {}),
        ...(direction ? { direction } : {}),
        ...(mediaType
          ? room.category === "prime"
            ? { body_type: mediaType }
            : { msg_type: mediaType === "image" ? 2 : 3 }
          : {}),
      }),
    },
  );
  return mapPage(unwrapResponse(response.data), room);
}

export async function searchChatMessages({
  room,
  keyword,
  cursorId = 0,
  pageSize = 50,
}: SearchChatMessagesParams): Promise<ChatMessagesPage> {
  const response = await axios.get<ApiResponse<BackendMessagesPage<BackendMessage | BackendPrimeMessage>>>(
    endpointFor(room, "search"),
    {
      params: freshRequestParams({
        ...roomParams(room),
        keyword,
        page_size: pageSize,
        ...(cursorId > 0 ? { cursor_id: cursorId } : {}),
      }),
    },
  );
  return mapPage(unwrapResponse(response.data), room);
}

export interface SendMessageParams {
  chatRoomId: string;
  content: string;
}

interface SendChatMessageResponse {
  chat_message_id?: string;
}

export async function sendChatMessage({
  chatRoomId,
  content,
}: SendMessageParams): Promise<string> {
  const response = await axios.post<ApiResponse<SendChatMessageResponse>>("/api/chat/send", {
    chat_room_id: chatRoomId,
    content,
  });
  const data = unwrapResponse(response.data);
  if (!data.chat_message_id) {
    throw new Error("The server did not return a chat message ID");
  }
  return data.chat_message_id;
}

export async function fetchUserProfile(): Promise<UserProfile> {
  const response = await axios.get<ApiResponse<BackendUserProfile>>("/api/user/me", {
    params: freshRequestParams({}),
  });
  const data = unwrapResponse(response.data);
  return {
    userId: data.user_id,
    uniqueId: data.unique_id,
    displayName: data.display_name,
    avatarUrl: data.avatar_url,
    sendChatEnabled: data.send_chat,
  };
}

export async function fetchRoomAllMedia(room: ChatRoom): Promise<MediaItem[]> {
  const [images, videos] = await Promise.all([
    fetchChatMessages({ room, pageSize: 1000, mediaType: "image" }),
    fetchChatMessages({ room, pageSize: 1000, mediaType: "video" }),
  ]);
  const mediaItems: MediaItem[] = [];
  for (const message of [...images.messages, ...videos.messages]) {
    if (!message.mediaUrl) continue;
    mediaItems.push({
      id: `media-${message.id}`,
      type: message.type === "video" ? "video" : "image",
      url: message.mediaUrl,
      fallbackUrl: message.mediaFallbackUrl,
      createdAt: message.createdAt,
      senderName: message.senderName,
      messageId: message.id,
      backendId: message.backendId,
    });
  }
  return mediaItems.sort(
    (a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime(),
  );
}

export async function fetchRoomAvailableDates(room: ChatRoom): Promise<string[]> {
  const response = await axios.get<ApiResponse<string[]>>(endpointFor(room, "dates"), {
    params: freshRequestParams(roomParams(room)),
  });
  const data = unwrapResponse(response.data);
  return Array.isArray(data) ? data.filter(Boolean) : [];
}

