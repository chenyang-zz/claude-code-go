// Package datetimeparser converts natural language dates and times into
// ISO 8601 format via the Haiku helper. The Go port mirrors
// src/utils/mcp/dateTimeParser.ts and is wired via the haiku.Querier interface.
//
// All prompt literals in this file are kept verbatim against the TS source
// to preserve model behaviour.
package datetimeparser

// systemPrompt is the system message used for date/time parsing.
// Verbatim copy from dateTimeParser.ts.
const systemPrompt = `You are a date/time parser that converts natural language into ISO 8601 format.
You MUST respond with ONLY the ISO 8601 formatted string, with no explanation or additional text.
If the input is ambiguous, prefer future dates over past dates.
For times without dates, use today's date.
For dates without times, do not include a time component.
If the input is incomplete or you cannot confidently parse it into a valid date, respond with exactly "INVALID" (nothing else).
Examples of INVALID input: partial dates like "2025-01-", lone numbers like "13", gibberish.
Examples of valid natural language: "tomorrow", "next Monday", "jan 1st 2025", "in 2 hours", "yesterday".`

// querySource is the identifier passed to the haiku layer for log
// correlation. Mirrors the TS querySource string verbatim.
const querySource = "mcp_datetime_parse"

// dateFormatDescription describes the expected output format for date-only parsing.
const dateFormatDescription = "YYYY-MM-DD (date only, no time)"

// dateTimeFormatDescription describes the expected output format for full date-time parsing.
const dateTimeFormatDescriptionTemplate = "YYYY-MM-DDTHH:MM:SS%s (full date-time with timezone)"

// userPromptTemplate is the template for the user prompt sent to Haiku.
// Substitution order: currentDateTime, timezone, dayOfWeek, input, formatDescription.
const userPromptTemplate = `Current context:
- Current date and time: %s (UTC)
- Local timezone: %s
- Day of week: %s

User input: "%s"

Output format: %s

Parse the user's input into ISO 8601 format. Return ONLY the formatted string, or "INVALID" if the input is incomplete or unparseable.`
