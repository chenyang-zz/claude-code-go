package ask_user_question

import (
	"context"
	"fmt"
	"strings"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

const (
	// Name is the stable registry identifier for the AskUserQuestion tool.
	Name = "AskUserQuestion"
	// headerMaxLength mirrors the TS ASK_USER_QUESTION_TOOL_CHIP_WIDTH.
	headerMaxLength = 12
	// maxQuestions mirrors the TS input schema max of 4.
	maxQuestions = 4
	// minOptions mirrors the TS option schema min of 2.
	minOptions = 2
	// maxOptions mirrors the TS option schema max of 4.
	maxOptions = 4
)

// toolDescription is the tool summary exposed to the model. It includes key usage notes that in the TS codebase
// are split across description() and prompt() — Go's Tool interface uses Description() as the single model-facing text.
const toolDescription = `Asks the user multiple choice questions to gather information, clarify ambiguity, understand preferences, make decisions or offer them choices.

Usage notes:
- Users will always be able to select "Other" to provide custom text input
- Use multiSelect: true to allow multiple answers to be selected for a question
- If you recommend a specific option, make that the first option in the list and add "(Recommended)" at the end of the label
- The optional preview field on options can present concrete artifacts (mockups, code snippets, diagrams) as markdown in a monospace box. Previews are only supported for single-select questions (not multiSelect).`

// QuestionOption represents one choice within a question presented to the user.
type QuestionOption struct {
	// Label is the display text for this option (1-5 words).
	Label string `json:"label"`
	// Description explains what this option means or what will happen if chosen.
	Description string `json:"description"`
	// Preview is optional content rendered when this option is focused.
	Preview string `json:"preview,omitempty"`
}

// Question represents a single question to ask the user.
type Question struct {
	// Question is the complete question text, ending with a question mark.
	Question string `json:"question"`
	// Header is a very short label displayed as a chip/tag (max 12 chars).
	Header string `json:"header"`
	// Options are the available choices (2-4 options).
	Options []QuestionOption `json:"options"`
	// MultiSelect allows multiple answers when true.
	MultiSelect bool `json:"multiSelect"`
}

// Annotation stores per-question metadata from the user.
type Annotation struct {
	// Preview is the preview content of the selected option.
	Preview string `json:"preview,omitempty"`
	// Notes is free-text notes the user added to their selection.
	Notes string `json:"notes,omitempty"`
}

// InputMetadata carries optional tracking information.
type InputMetadata struct {
	// Source identifies the origin of the question (e.g. "remember").
	Source string `json:"source,omitempty"`
}

// Input is the typed request payload for the AskUserQuestion tool.
type Input struct {
	// Questions is the list of questions to ask (1-4).
	Questions []Question `json:"questions"`
	// Answers contains pre-collected user answers keyed by question text.
	Answers map[string]string `json:"answers,omitempty"`
	// Annotations contains per-question user annotations.
	Annotations map[string]Annotation `json:"annotations,omitempty"`
	// Metadata carries optional tracking information.
	Metadata *InputMetadata `json:"metadata,omitempty"`
}

// Output is the structured result returned by the AskUserQuestion tool.
type Output struct {
	// Questions is the list of questions that were asked.
	Questions []Question `json:"questions"`
	// Answers maps question text to the user's answer string.
	Answers map[string]string `json:"answers"`
	// Annotations contains per-question user annotations.
	Annotations map[string]Annotation `json:"annotations,omitempty"`
}

// Tool implements the minimum migrated AskUserQuestion tool.
type Tool struct{}

// NewTool constructs an AskUserQuestion tool instance.
func NewTool() *Tool {
	return &Tool{}
}

// Name returns the stable registration name.
func (t *Tool) Name() string {
	return Name
}

// Description returns the tool summary exposed to provider tool schemas.
func (t *Tool) Description() string {
	return toolDescription
}

// InputSchema returns the AskUserQuestion input contract.
func (t *Tool) InputSchema() coretool.InputSchema {
	return inputSchema()
}

// IsReadOnly reports that AskUserQuestion never mutates external state.
func (t *Tool) IsReadOnly() bool {
	return true
}

// IsConcurrencySafe reports that independent invocations may run in parallel safely.
func (t *Tool) IsConcurrencySafe() bool {
	return true
}

// RequiresUserInteraction reports that this tool requires user input before the model can continue.
func (t *Tool) RequiresUserInteraction() bool {
	return true
}

// Invoke validates the input and returns a pass-through result containing the questions and answers.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("ask_user_question tool: nil receiver")
	}

	input, err := coretool.DecodeInput[Input](inputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	if err := validateQuestions(input.Questions); err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	output := Output{
		Questions:   input.Questions,
		Answers:     input.Answers,
		Annotations: input.Annotations,
	}
	if output.Answers == nil {
		output.Answers = map[string]string{}
	}

	return coretool.Result{
		Output: formatOutputText(output),
		Meta: map[string]any{
			"data": output,
		},
	}, nil
}

// inputSchema builds the declared input schema exposed to model providers.
func inputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"questions": {
				Type:        coretool.ValueKindArray,
				Description: "Questions to ask the user (1-4 questions). Each question must have: question (string), header (string, max 12 chars), options (array of 2-4 {label, description, preview?}), multiSelect (boolean, default false).",
				Required:    true,
				Items: &coretool.FieldSchema{
					Type:        coretool.ValueKindObject,
					Description: "A question object with question, header, options, and multiSelect fields.",
				},
			},
			"answers": {
				Type:        coretool.ValueKindObject,
				Description: "User answers collected by the permission component. Maps question text to answer string.",
			},
			"annotations": {
				Type:        coretool.ValueKindObject,
				Description: "Optional per-question annotations from the user (e.g., notes on preview selections). Keyed by question text.",
			},
			"metadata": {
				Type:        coretool.ValueKindObject,
				Description: "Optional metadata for tracking purposes. Contains an optional 'source' string field.",
			},
		},
	}
}

// validateQuestions enforces the TS-side uniqueness and length constraints.
func validateQuestions(questions []Question) error {
	if len(questions) == 0 {
		return fmt.Errorf("at least 1 question is required")
	}
	if len(questions) > maxQuestions {
		return fmt.Errorf("at most %d questions are allowed, got %d", maxQuestions, len(questions))
	}

	// Question texts must be unique.
	seenQuestions := make(map[string]bool, len(questions))
	for i, q := range questions {
		if q.Question == "" {
			return fmt.Errorf("questions[%d].question is required", i)
		}
		if len(q.Header) > headerMaxLength {
			return fmt.Errorf("questions[%d].header must be at most %d characters, got %d", i, headerMaxLength, len(q.Header))
		}
		if seenQuestions[q.Question] {
			return fmt.Errorf("question texts must be unique: %q appears more than once", q.Question)
		}
		seenQuestions[q.Question] = true

		// Option validation.
		if len(q.Options) < minOptions {
			return fmt.Errorf("questions[%d] must have at least %d options, got %d", i, minOptions, len(q.Options))
		}
		if len(q.Options) > maxOptions {
			return fmt.Errorf("questions[%d] must have at most %d options, got %d", i, maxOptions, len(q.Options))
		}

		// Option labels must be unique within each question.
		seenLabels := make(map[string]bool, len(q.Options))
		for j, opt := range q.Options {
			if opt.Label == "" {
				return fmt.Errorf("questions[%d].options[%d].label is required", i, j)
			}
			if opt.Description == "" {
				return fmt.Errorf("questions[%d].options[%d].description is required", i, j)
			}
			if seenLabels[opt.Label] {
				return fmt.Errorf("option labels must be unique within each question: %q in question %q appears more than once", opt.Label, q.Question)
			}
			seenLabels[opt.Label] = true
		}
	}

	return nil
}

// formatOutputText builds a human-readable summary of the answers for the model.
func formatOutputText(output Output) string {
	if len(output.Answers) == 0 {
		return "No answers were provided by the user."
	}

	var b strings.Builder
	b.WriteString("User has answered your questions: ")
	first := true
	for questionText, answer := range output.Answers {
		if !first {
			b.WriteString(", ")
		}
		first = false
		fmt.Fprintf(&b, "%q=%q", questionText, answer)
	}
	b.WriteString(". You can now continue with the user's answers in mind.")
	return b.String()
}
