export type LineType = "delta" | "thinking" | "tool" | "error" | "info" | "line" | "code";

export interface Line {
  id: number;
  text: string;
  type: LineType;
  codeLanguage?: string;
}

export type TypewriterKind = "delta" | "thinking";

export interface PermissionDialogState {
  visible: boolean;
  title: string;
  body: string;
}

export interface TokenState {
  inputTokens?: number;
  outputTokens?: number;
}
