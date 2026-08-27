/**
 * Replive Fandom 订阅天数与字符限制规则工具
 * 逆向依据：
 * - 胶囊颜色与爱心阶梯：APK xo/k.java (DayCountChip)
 * - 订阅天数字数限制阶梯：APK mu/d1.java
 */

export interface DayCountStyle {
  color: string;
  bgColor: string;
  borderColor: string;
  heartSize: number; // in pixels
}

/**
 * 根据 Fandom 订阅天数计算对应的胶囊样式配置
 */
export function getDayCountStyle(dayCount: number): DayCountStyle {
  if (dayCount >= 500) {
    return {
      color: "#E23122",
      bgColor: "rgba(226, 49, 34, 0.12)",
      borderColor: "rgba(226, 49, 34, 0.35)",
      heartSize: 18,
    };
  }
  if (dayCount >= 400) {
    return {
      color: "#EA642B",
      bgColor: "rgba(234, 100, 43, 0.12)",
      borderColor: "rgba(234, 100, 43, 0.35)",
      heartSize: 17,
    };
  }
  if (dayCount >= 300) {
    return {
      color: "#F3B03E",
      bgColor: "rgba(243, 176, 62, 0.12)",
      borderColor: "rgba(243, 176, 62, 0.35)",
      heartSize: 16,
    };
  }
  if (dayCount >= 200) {
    return {
      color: "#5FC83E",
      bgColor: "rgba(95, 200, 62, 0.12)",
      borderColor: "rgba(95, 200, 62, 0.35)",
      heartSize: 15,
    };
  }
  if (dayCount >= 100) {
    return {
      color: "#388AF1",
      bgColor: "rgba(56, 138, 241, 0.12)",
      borderColor: "rgba(56, 138, 241, 0.35)",
      heartSize: 14,
    };
  }
  if (dayCount >= 30) {
    return {
      color: "#6028EE",
      bgColor: "rgba(96, 40, 238, 0.12)",
      borderColor: "rgba(96, 40, 238, 0.35)",
      heartSize: 13,
    };
  }
  // 1 ~ 29 天
  return {
    color: "#9CA3AF",
    bgColor: "rgba(156, 163, 175, 0.12)",
    borderColor: "rgba(156, 163, 175, 0.30)",
    heartSize: 12,
  };
}

/**
 * 根据 Fandom 订阅天数计算发送消息的最大字数限制 (UTF-16 Code Unit)
 * APK mu/d1.java 算法确认：
 * 1～9 天: 30
 * 10～29 天: 40
 * 30～99 天: 50
 * 100～199 天: 60
 * 200～299 天: 70
 * 300～399 天: 80
 * 400～499 天: 90
 * 500+ 天: 100
 */
export function getMaxCharacterLimit(dayCount?: number): number {
  if (typeof dayCount !== "number" || dayCount < 1) {
    return 30;
  }
  if (dayCount < 10) return 30;
  if (dayCount < 30) return 40;
  if (dayCount < 100) return 50;
  if (dayCount < 200) return 60;
  if (dayCount < 300) return 70;
  if (dayCount < 400) return 80;
  if (dayCount < 500) return 90;
  return 100;
}

/**
 * 获取字符串的 UTF-16 Code Unit 长度 (与 Java String.length() 保持一致)
 */
export function getUtf16Length(text: string): number {
  return text.length;
}
