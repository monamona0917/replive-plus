import type { ChatRoom, Message, UserProfile } from "../types/chat";

export const MOCK_USER_PROFILE: UserProfile = {
  userId: "user_me_001",
  uniqueId: "prime_user_vip",
  displayName: "管理员用户",
  avatarUrl: "https://api.dicebear.com/7.x/bottts/svg?seed=admin_avatar",
  sendChatEnabled: true,
};

export const MOCK_ROOMS: ChatRoom[] = [
  {
    id: 1,
    userId: "fandom_main_01",
    chatRoomId: "room_fandom_live",
    displayName: "Fandom 频道",
    avatarUrl: "https://api.dicebear.com/7.x/shapes/svg?seed=fandom_star",
    category: "fandom",
    lastMessageTime: new Date(Date.now() - 1000 * 60 * 5).toISOString(),
    lastMessageContent: "欢迎来到 Fandom 聊天记录归档大厅！",
    messageCount: 1248,
  },
  {
    id: 2,
    userId: "fandom_channel_02",
    chatRoomId: "room_fandom_announcements",
    displayName: "Fandom 公告与精选",
    avatarUrl: "https://api.dicebear.com/7.x/shapes/svg?seed=fandom_bell",
    category: "fandom",
    lastMessageTime: new Date(Date.now() - 1000 * 60 * 60 * 2).toISOString(),
    lastMessageContent: "本周直播回放与高光切片已同步。",
    messageCount: 382,
  },
  {
    id: 3,
    userId: "prime_user_alice",
    chatRoomId: "room_prime_alice",
    displayName: "Alice (Prime)",
    avatarUrl: "https://api.dicebear.com/7.x/adventurer/svg?seed=Alice",
    category: "prime",
    lastMessageTime: new Date(Date.now() - 1000 * 60 * 30).toISOString(),
    lastMessageContent: "今天晚上有特别的互动环节哦~",
    messageCount: 520,
  },
  {
    id: 4,
    userId: "prime_user_hikari",
    chatRoomId: "room_prime_hikari",
    displayName: "光 (Hikari Prime)",
    avatarUrl: "https://api.dicebear.com/7.x/adventurer/svg?seed=Hikari",
    category: "prime",
    lastMessageTime: new Date(Date.now() - 1000 * 60 * 120).toISOString(),
    lastMessageContent: "非常感谢大家的信件与打赏！",
    messageCount: 890,
  },
  {
    id: 5,
    userId: "prime_user_kuro",
    chatRoomId: "room_prime_kuro",
    displayName: "Kuro (Prime 专属)",
    avatarUrl: "https://api.dicebear.com/7.x/adventurer/svg?seed=Kuro",
    category: "prime",
    lastMessageTime: new Date(Date.now() - 1000 * 60 * 60 * 24).toISOString(),
    lastMessageContent: "明天见，晚安！",
    messageCount: 412,
  },
];

export function generateMockMessages(room: ChatRoom): Message[] {
  const isFandom = room.category === "fandom" || room.displayName.includes("Fandom");
  const now = Date.now();
  const oneHour = 1000 * 60 * 60;
  const oneDay = oneHour * 24;

  const yesterday = now - oneDay;
  const twoDaysAgo = now - oneDay * 2;

  const baseSender = isFandom ? "Fandom Host" : room.displayName;
  const baseAvatar = room.avatarUrl;

  return [
    {
      id: "msg_mock_01",
      backendId: 101,
      chatMessageId: "chat_001",
      content: isFandom
        ? "【DB 归档记录】Fandom 历史会话数据库载入成功。"
        : `你好！这是与 ${room.displayName} 的 Prime Chat 历史记录。`,
      type: "text",
      createdAt: new Date(twoDaysAgo + oneHour * 10).toISOString(),
      senderId: room.userId,
      senderName: baseSender,
    },
    {
      id: "msg_mock_02",
      backendId: 102,
      chatMessageId: "chat_002",
      content: "系统支持向上滚动无缝加载更早历史，支持日期快速穿越与多语种机翻。",
      type: "text",
      createdAt: new Date(twoDaysAgo + oneHour * 10 + 1000 * 45).toISOString(),
      senderId: room.userId,
      senderName: baseSender,
    },
    {
      id: "msg_mock_03",
      backendId: 103,
      chatMessageId: "chat_003",
      content: "收到！已经能正常查阅历史记录和图片视频了。",
      type: "text",
      createdAt: new Date(twoDaysAgo + oneHour * 11).toISOString(),
      senderId: "user_me_001",
      senderName: "我",
    },
    {
      id: "msg_mock_04",
      backendId: 104,
      chatMessageId: "chat_004",
      content: "今天的天气真不错，分享一张现场照片给大家！",
      type: "image",
      mediaUrl: "https://images.unsplash.com/photo-1534447677768-be436bb09401?w=800&auto=format&fit=crop&q=80",
      createdAt: new Date(yesterday + oneHour * 14).toISOString(),
      senderId: room.userId,
      senderName: baseSender,
    },
    {
      id: "msg_mock_05",
      backendId: 105,
      chatMessageId: "chat_005",
      content: "今日も一日お疲れ様でした！明日も頑張りましょう。",
      type: "text",
      createdAt: new Date(yesterday + oneHour * 20).toISOString(),
      senderId: room.userId,
      senderName: baseSender,
    },
    {
      id: "msg_mock_06",
      backendId: 106,
      chatMessageId: "chat_006",
      content: "辛苦了！期待下一次精彩内容。",
      type: "text",
      createdAt: new Date(yesterday + oneHour * 20 + 1000 * 30).toISOString(),
      senderId: "user_me_001",
      senderName: "我",
    },
    {
      id: "msg_mock_07",
      backendId: 107,
      chatMessageId: "chat_007",
      content: "最新一条同步消息：欢迎使用 Replive Web Pro 重构版！",
      type: "text",
      createdAt: new Date(now - 1000 * 60 * 10).toISOString(),
      senderId: room.userId,
      senderName: baseSender,
    },
  ];
}
