export type ChatCategory = "fandom" | "prime";

export interface User {
  id: string;
  name: string;
  avatar?: string;
}

export interface ChatRoom {
  id?: number;
  userId: string;
  uniqueId?: string;
  chatRoomId: string;
  displayName: string;
  avatarUrl?: string;
  memberUserId?: string;
  backgroundImageUrl?: string;
  // Fandom talent's last room-check time, normalized to Unix milliseconds.
  talentLastCheckTime?: number;
  category?: ChatCategory;
  lastMessageTime?: string;
  lastMessageContent?: string;
  lastMessageType?: "text" | "image" | "video";
  messageCount?: number;
}

export interface Message {
  id: string;
  backendId: number;
  chatMessageId: string;
  content: string;
  type: "text" | "image" | "video";
  createdAt: string;
  mediaUrl?: string;
  mediaFallbackUrl?: string;
  senderId: string;
  senderName: string;
  senderKind?: "talent" | "member";
  reactionEmoji?: string;
  isDeleted?: boolean;
  coinAmount?: number;
}

export interface MessageGroup {
  date: string;
  messages: Message[];
}

export interface UserProfile {
  userId: string;
  uniqueId: string;
  displayName: string;
  avatarUrl?: string;
  sendChatEnabled?: boolean;
}

export interface TranslationState {
  text?: string;
  loading?: boolean;
  error?: string;
  visible?: boolean;
  targetLang?: string;
}

export interface BackendChatRoom {
  id?: number;
  user_id: string;
  unique_id?: string;
  chat_room_id: string;
  display_name: string;
  avatar_url?: string;
  talent_last_check_time?: number | string;
  last_message_time?: number;
  last_message_content?: string;
  last_message_type?: number;
}

export interface BackendUserProfile {
  user_id: string;
  unique_id: string;
  display_name: string;
  avatar_url?: string;
  send_chat?: boolean;
}

export interface BackendMessage {
  id: number;
  user_id: string;
  display_name: string;
  chat_room_id: string;
  chat_message_id: string;
  msg_type: number;
  content: string;
  image_url?: string;
  video_url?: string;
  image_local_url?: string;
  video_local_url?: string;
  send_time?: number;
  time_str?: string;
}

export interface BackendPrimeChatRoom {
  id?: number;
  chat_room_id: string;
  talent_user_id: string;
  talent_unique_id?: string;
  talent_display_name?: string;
  talent_avatar_url?: string;
  member_user_id?: string;
  member_background_image_url?: string;
  last_message_time?: number;
  last_message_content?: string;
  last_message_type?: "text" | "image" | "video" | "unknown";
}

export interface BackendPrimeMessage {
  id: number;
  message_id: string;
  chat_room_id: string;
  chat_room_owner_user_id?: string;
  member_user_id?: string;
  sender?: "talent" | "member" | "unknown";
  body_type?: "text" | "image" | "video" | "unknown";
  content?: string;
  image_url?: string;
  video_url?: string;
  coin_amount?: number;
  reaction_emoji?: string;
  is_deleted?: boolean;
  create_unix_time_ms?: number;
}

export interface ChatMessagesPage {
  messages: Message[];
  nextCursorId: number;
  prevCursorId: number;
  hasMore: boolean;
  hasOlder: boolean;
  hasNewer: boolean;
  anchorId: number;
}

export interface MediaItem {
  id: string;
  type: "image" | "video";
  url: string;
  fallbackUrl?: string;
  createdAt: string;
  senderName: string;
  messageId: string;
  backendId: number;
}
