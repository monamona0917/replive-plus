import { AlertCircle, X } from "lucide-react";
import useChatStore from "../../stores/chat-store";
import ChatHeader from "./ChatHeader";
import ChatInput from "./ChatInput";
import ChatList from "./ChatList";

export const ChatContainer = () => {
  const error = useChatStore((s) => s.error);
  const setError = useChatStore((s) => s.setError);

  return (
    <main className="flex-1 flex flex-col h-full bg-background relative overflow-hidden">
      {/* Header */}
      <ChatHeader />

      {/* Error notification banner if any */}
      {error && (
        <div className="flex items-center justify-between px-4 py-2 bg-destructive/10 border-b border-destructive/20 text-destructive text-xs transition-all">
          <div className="flex items-center gap-2">
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span>{error}</span>
          </div>
          <button
            type="button"
            onClick={() => setError(null)}
            className="p-1 rounded hover:bg-destructive/15 transition-colors"
          >
            <X className="w-3.5 h-3.5" />
          </button>
        </div>
      )}

      {/* Chat Messages Stream */}
      <ChatList />

      {/* Footer Input */}
      <ChatInput />
    </main>
  );
};

export default ChatContainer;
