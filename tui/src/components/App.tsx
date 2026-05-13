import React from "react";
import { Box, Text, useApp } from "ink";
import { Chat } from "./Chat.js";
import { Input } from "./Input.js";
import { PermissionDialog } from "./PermissionDialog.js";
import { StatusLine } from "./StatusLine.js";
import { useTuiSession } from "../hooks/useTuiSession.js";

interface AppProps {
  port: number;
}

export function App({ port }: AppProps) {
  useApp();
  const session = useTuiSession(port);

  return (
    <Box flexDirection="column" height="100%">
      <Box flexGrow={1} flexDirection="column">
        <Box marginBottom={1}>
          <Text bold>
            Claude Code {session.connected ? <Text color="green">● Connected</Text> : <Text color="red">● Disconnected</Text>}
          </Text>
        </Box>
        <Chat lines={session.lines} isThinking={session.isThinking} />
        <PermissionDialog
          visible={session.permission.visible}
          title={session.permission.title}
          body={session.permission.body}
          onApprove={session.handleApprove}
          onDeny={session.handleDeny}
        />
      </Box>
      <Box flexDirection="column">
        <Box>
          <Input onSubmit={session.submit} disabled={session.inputDisabled || session.permission.visible} />
        </Box>
        <Box marginTop={1}>
          <StatusLine
            connected={session.connected}
            isRunning={session.isRunning}
            isThinking={session.isThinking && !session.permission.visible}
            inputTokens={session.tokens.inputTokens}
            outputTokens={session.tokens.outputTokens}
          />
        </Box>
      </Box>
    </Box>
  );
}
