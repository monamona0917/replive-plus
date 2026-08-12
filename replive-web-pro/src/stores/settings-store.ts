import { create } from "zustand";
import type { SettingsState, ThemeMode, WatermarkConfig } from "../types/settings";

const STORAGE_KEY = "replive_plus_settings";

interface PersistedSettings {
  theme: ThemeMode;
  watermark: WatermarkConfig;
  sidebarCollapsed: boolean;
}

const DEFAULT_WATERMARK: WatermarkConfig = {
  enabled: true,
  text: "",
  opacity: 0.12,
  angle: -22,
  fontSize: 14,
};

const DEFAULT_SETTINGS: PersistedSettings = {
  theme: "dark",
  watermark: DEFAULT_WATERMARK,
  sidebarCollapsed: false,
};

function loadSettings(): PersistedSettings {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return DEFAULT_SETTINGS;
    const parsed = JSON.parse(raw);
    const theme: ThemeMode = parsed.theme === "light" ? "light" : "dark";
    return {
      theme,
      watermark: {
        ...DEFAULT_WATERMARK,
        ...(parsed.watermark || {}),
        enabled: true, // 强制开启水印
      },
      sidebarCollapsed: Boolean(parsed.sidebarCollapsed),
    };
  } catch {
    return DEFAULT_SETTINGS;
  }
}

function saveSettings(settings: PersistedSettings) {
  try {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        ...settings,
        watermark: { ...settings.watermark, enabled: true },
      }),
    );
  } catch (err) {
    console.error("Failed to save settings to localStorage:", err);
  }
}

function applyThemeToDocument(theme: ThemeMode) {
  const root = document.documentElement;
  if (theme === "dark") {
    root.classList.add("dark");
  } else {
    root.classList.remove("dark");
  }
}

const initialSettings = loadSettings();
applyThemeToDocument(initialSettings.theme);

export const useSettingsStore = create<SettingsState>((set, get) => ({
  theme: initialSettings.theme,
  watermark: initialSettings.watermark,
  sidebarCollapsed: initialSettings.sidebarCollapsed,

  setTheme: (theme: ThemeMode) => {
    applyThemeToDocument(theme);
    set({ theme });
    saveSettings({
      theme,
      watermark: { ...get().watermark, enabled: true },
      sidebarCollapsed: get().sidebarCollapsed,
    });
  },

  updateWatermark: (partial: Partial<WatermarkConfig>) => {
    const nextWatermark: WatermarkConfig = {
      ...get().watermark,
      ...partial,
      enabled: true, // 强制开启
    };
    set({ watermark: nextWatermark });
    saveSettings({
      theme: get().theme,
      watermark: nextWatermark,
      sidebarCollapsed: get().sidebarCollapsed,
    });
  },

  setSidebarCollapsed: (collapsed: boolean) => {
    set({ sidebarCollapsed: collapsed });
    saveSettings({
      theme: get().theme,
      watermark: { ...get().watermark, enabled: true },
      sidebarCollapsed: collapsed,
    });
  },

  toggleSidebar: () => {
    const next = !get().sidebarCollapsed;
    set({ sidebarCollapsed: next });
    saveSettings({
      theme: get().theme,
      watermark: { ...get().watermark, enabled: true },
      sidebarCollapsed: next,
    });
  },
}));

export default useSettingsStore;
