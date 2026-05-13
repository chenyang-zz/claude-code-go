import React, { useState, useCallback } from "react";
import { Box, Text, useInput } from "ink";

interface PermissionDialogProps {
  visible: boolean;
  title: string;
  body: string;
  onApprove: () => void;
  onDeny: () => void;
}

export function PermissionDialog({
  visible,
  title,
  body,
  onApprove,
  onDeny,
}: PermissionDialogProps) {
  const [focus, setFocus] = useState<"approve" | "deny">("deny");

  useInput(
    (input, key) => {
      if (!visible) return;

      if (key.tab) {
        setFocus((f) => (f === "approve" ? "deny" : "approve"));
        return;
      }

      if (key.return) {
        if (focus === "approve") {
          onApprove();
        } else {
          onDeny();
        }
        return;
      }

      if (input === "y" || input === "Y") {
        onApprove();
        return;
      }

      if (input === "n" || input === "N" || key.escape) {
        onDeny();
        return;
      }
    },
  );

  if (!visible) return null;

  return (
    <Box
      borderStyle="round"
      borderColor="yellow"
      flexDirection="column"
      paddingX={1}
      marginY={1}
    >
      <Box>
        <Text bold color="yellow">
          {title}
        </Text>
      </Box>
      {body ? (
        <Box>
          <Text color="white">{body}</Text>
        </Box>
      ) : null}
      <Box marginTop={1}>
        <Text>
          {focus === "approve" ? (
            <Text bold color="green">
              {" "}[Y] Approve{" "}
            </Text>
          ) : (
            <Text dimColor>
              {" "} y/Enter to Approve{" "}
            </Text>
          )}
          <Text>  </Text>
          {focus === "deny" ? (
            <Text bold color="red">
              [N] Deny
            </Text>
          ) : (
            <Text dimColor>
              n/Esc to Deny
            </Text>
          )}
        </Text>
      </Box>
    </Box>
  );
}
