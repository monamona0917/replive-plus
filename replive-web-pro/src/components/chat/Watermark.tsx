import { useMemo } from "react";
import useChatStore from "../../stores/chat-store";
import useSettingsStore from "../../stores/settings-store";

export const Watermark = () => {
  const watermark = useSettingsStore((s) => s.watermark);
  const theme = useSettingsStore((s) => s.theme);
  const selectedRoom = useChatStore((s) => s.selectedRoom);

  const displayText = selectedRoom?.displayName?.trim() || "";

  const svgDataUri = useMemo(() => {
    const angle = watermark.angle ?? -22;
    const fontSize = watermark.fontSize ?? 14;
    const opacity = watermark.opacity ?? 0.12;
    // 深色模式下用亮白文字，浅色模式下用暗黑文字以保证清晰度
    const fillHex = theme === "dark" ? "#FFFFFF" : "#000000";

    const svgString = `<svg xmlns="http://www.w3.org/2000/svg" width="280" height="180">
      <text x="50%" y="50%" fill="${fillHex}" fill-opacity="${opacity}" 
        font-family="system-ui, -apple-system, sans-serif" font-size="${fontSize}" font-weight="600" 
        text-anchor="middle" dominant-baseline="middle" 
        transform="rotate(${angle} 140 90)">
        ${displayText.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")}
      </text>
    </svg>`;

    return `data:image/svg+xml;utf8,${encodeURIComponent(svgString)}`;
  }, [watermark.angle, watermark.fontSize, watermark.opacity, displayText, theme]);

  if (!displayText) return null;

  return (
    <div
      aria-hidden="true"
      className="pointer-events-none fixed inset-0 z-40 select-none overflow-hidden"
      style={{
        backgroundImage: `url("${svgDataUri}")`,
        backgroundRepeat: "repeat",
        backgroundSize: "280px 180px",
      }}
    />
  );
};

export default Watermark;
