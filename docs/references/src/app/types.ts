export type FileNode = {
  name: string;
  type: "file" | "folder";
  children?: FileNode[];
  content?: string;
  language?: string;
};

export type Message = {
  id: string;
  role: "user" | "assistant";
  content: string;
  timestamp: string;
};

export type ChatSession = {
  id: string;
  name: string;
  messages: Message[];
};

export type Session = {
  id: string;
  title: string;
  files: FileNode[];
  chatSessions: ChatSession[];
  activeFile: FileNode | null;
  activeChatSession: ChatSession | null;
  createdAt: string;
};