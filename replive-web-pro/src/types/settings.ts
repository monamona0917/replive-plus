export type ThemeMode = "light" | "dark";

export interface WatermarkConfig {
  enabled: boolean;
  text: string;
  opacity: number;
  angle: number;
  fontSize: number;
}

export interface SettingsState {
  theme: ThemeMode;
  watermark: WatermarkConfig;
  sidebarCollapsed: boolean;
  setTheme: (theme: ThemeMode) => void;
  updateWatermark: (config: Partial<WatermarkConfig>) => void;
  setSidebarCollapsed: (collapsed: boolean) => void;
  toggleSidebar: () => void;
}
