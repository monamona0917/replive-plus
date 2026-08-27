import { ExternalLink, Info, X } from "lucide-react";
import useChatStore from "../../stores/chat-store";

export const AboutModal = () => {
  const aboutModalOpen = useChatStore((s) => s.aboutModalOpen);
  const setAboutModalOpen = useChatStore((s) => s.setAboutModalOpen);

  if (!aboutModalOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm animate-in fade-in duration-200 select-none">
      <div className="w-full max-w-md bg-card border border-border rounded-2xl shadow-2xl p-6 relative">
        {/* Close Button */}
        <button
          type="button"
          onClick={() => setAboutModalOpen(false)}
          className="absolute right-4 top-4 p-1.5 rounded-lg text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
        >
          <X className="w-4 h-4" />
        </button>

        {/* Modal Header */}
        <div className="flex items-center gap-2.5 mb-5">
          <div className="p-2 rounded-xl bg-primary/15 text-primary">
            <Info className="w-5 h-5" />
          </div>
          <h2 className="text-base font-bold text-foreground">关于 Replive+</h2>
        </div>

        {/* Modal Body */}
        <div className="space-y-4">
          <div className="p-4 rounded-xl bg-muted/40 border border-border/60 space-y-3 text-xs">
            <p className="text-muted-foreground leading-relaxed font-medium">
              项目地址：
            </p>
            <a
              href="https://github.com/monamona0917/replive-plus"
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center justify-between p-2.5 rounded-lg bg-card border border-border/70 text-foreground hover:text-primary hover:border-primary/50 transition-all font-medium group"
            >
              <div className="flex flex-col gap-0.5">
                <span className="font-semibold text-foreground group-hover:text-primary transition-colors">
                  monamona0917/replive-plus
                </span>
                <span className="text-[11px] text-muted-foreground">
                  すべては、この瞬間のために
                </span>
              </div>
              <ExternalLink className="w-3.5 h-3.5 text-muted-foreground group-hover:text-primary transition-colors shrink-0" />
            </a>

            <p className="text-muted-foreground leading-relaxed font-medium pt-1">
              本项目基于以下项目进行二次开发/重构筑：
            </p>
            <div className="space-y-2">
              <a
                href="https://github.com/Chilfish/replive-oyu"
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center justify-between p-2.5 rounded-lg bg-card border border-border/70 text-foreground hover:text-primary hover:border-primary/50 transition-all font-medium group"
              >
                <span>Chilfish/replive-oyu</span>
                <ExternalLink className="w-3.5 h-3.5 text-muted-foreground group-hover:text-primary transition-colors" />
              </a>

              <a
                href="https://github.com/huangwg2529/nsy_chat_live"
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center justify-between p-2.5 rounded-lg bg-card border border-border/70 text-foreground hover:text-primary hover:border-primary/50 transition-all font-medium group"
              >
                <span>huangwg2529/nsy_chat_live</span>
                <ExternalLink className="w-3.5 h-3.5 text-muted-foreground group-hover:text-primary transition-colors" />
              </a>
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="mt-6 flex justify-end">
          <button
            type="button"
            onClick={() => setAboutModalOpen(false)}
            className="px-5 py-2 rounded-xl text-xs font-semibold bg-primary text-primary-foreground hover:opacity-90 active:scale-95 transition-all shadow-sm"
          >
            完成
          </button>
        </div>
      </div>
    </div>
  );
};

export default AboutModal;
