import { Loader2, Search, X } from "lucide-react";
import { useEffect, useRef } from "react";
import { cn, formatTimeStr } from "../../lib/utils";
import useChatStore from "../../stores/chat-store";
import type { Message } from "../../types/chat";

export const SearchDrawer = () => {
  const searchDrawerOpen = useChatStore((s) => s.searchDrawerOpen);
  const setSearchDrawerOpen = useChatStore((s) => s.setSearchDrawerOpen);
  const searchQuery = useChatStore((s) => s.searchQuery);
  const searchResults = useChatStore((s) => s.searchResults);
  const isSearching = useChatStore((s) => s.isSearching);
  const searchMessages = useChatStore((s) => s.searchMessages);
  const clearSearch = useChatStore((s) => s.clearSearch);
  const jumpToMessage = useChatStore((s) => s.jumpToMessage);
  const selectedRoom = useChatStore((s) => s.selectedRoom);

  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (searchDrawerOpen) {
      setTimeout(() => inputRef.current?.focus(), 150);
    }
  }, [searchDrawerOpen]);

  if (!searchDrawerOpen) return null;

  const handleResultClick = (message: Message) => {
    void jumpToMessage(message, selectedRoom || undefined);
  };

  const renderHighlightedContent = (content: string, query: string) => {
    if (!query.trim()) return content;
    const parts = content.split(new RegExp(`(${query})`, "gi"));
    return parts.map((part, idx) =>
      part.toLowerCase() === query.toLowerCase() ? (
        <mark
          // biome-ignore lint/suspicious/noArrayIndexKey: simple snippet
          key={idx}
          className="bg-primary/25 text-primary font-semibold rounded-xs px-0.5"
        >
          {part}
        </mark>
      ) : (
        part
      ),
    );
  };

  return (
    <div className="fixed inset-0 z-50 overflow-hidden select-none animate-in fade-in duration-200">
      {/* Backdrop */}
      <div
        className="fixed inset-0 bg-black/50 backdrop-blur-xs transition-opacity"
        onClick={() => setSearchDrawerOpen(false)}
      />

      {/* Drawer Panel */}
      <div className="fixed inset-y-0 right-0 w-full sm:w-96 bg-card border-l border-border shadow-2xl flex flex-col z-50 animate-in slide-in-from-right duration-250">
        {/* Header */}
        <div className="flex items-center justify-between px-4 py-3.5 border-b border-border">
          <div className="flex items-center gap-2">
            <Search className="w-4 h-4 text-primary" />
            <h3 className="text-sm font-bold text-foreground">全文关键词检索</h3>
          </div>
          <button
            type="button"
            onClick={() => setSearchDrawerOpen(false)}
            className="p-1.5 rounded-lg text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Search Input Box */}
        <div className="p-3 border-b border-border/60 bg-muted/20">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
            <input
              ref={inputRef}
              type="text"
              value={searchQuery}
              onChange={(e) => void searchMessages(e.target.value)}
              placeholder="输入关键词实时搜索..."
              className="w-full pl-9 pr-8 py-2 text-xs bg-muted/60 border border-border rounded-xl outline-none focus:border-primary focus:bg-background transition-all"
            />
            {searchQuery && (
              <button
                type="button"
                onClick={clearSearch}
                className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
              >
                <X className="w-3.5 h-3.5" />
              </button>
            )}
          </div>

          <div className="flex items-center justify-between mt-2 text-[11px] text-muted-foreground px-1">
            <span>
              {searchQuery.trim()
                ? `共找到 ${searchResults.length} 条匹配记录`
                : "支持搜索文本、时间与发送者"}
            </span>
            {isSearching && <Loader2 className="w-3 h-3 animate-spin text-primary" />}
          </div>
        </div>

        {/* Results List */}
        <div className="flex-1 overflow-y-auto p-2 space-y-1.5">
          {searchQuery.trim() && searchResults.length === 0 && !isSearching && (
            <div className="py-16 text-center text-xs text-muted-foreground">
              未搜索到包含 “{searchQuery}” 的消息记录
            </div>
          )}

          {searchResults.map((message) => (
            <button
              type="button"
              key={message.id}
              onClick={() => handleResultClick(message)}
              className="w-full p-2.5 rounded-xl border border-border/40 bg-muted/20 hover:bg-muted hover:border-primary/40 text-left transition-all group"
            >
              <div className="flex items-center justify-between mb-1 text-[11px]">
                <span className="font-semibold text-foreground group-hover:text-primary transition-colors">
                  {message.senderName}
                </span>
                <span className="text-[10px] text-muted-foreground font-mono">
                  {formatTimeStr(message.createdAt)}
                </span>
              </div>
              <p className="text-xs text-foreground/80 line-clamp-2 leading-relaxed break-words">
                {renderHighlightedContent(message.content, searchQuery)}
              </p>
            </button>
          ))}
        </div>
      </div>
    </div>
  );
};

export default SearchDrawer;
