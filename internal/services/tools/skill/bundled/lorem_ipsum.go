// Package bundled registers the bundled (built-in) skills that ship with the CLI.
package bundled

import (
	"math"
	"math/rand/v2"
	"strconv"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/services/tools/skill"
)

// oneTokenWords are verified 1-token words for filler text generation.
var oneTokenWords = []string{
	"the", "a", "an", "I", "you", "he", "she", "it", "we", "they",
	"me", "him", "her", "us", "them", "my", "your", "his", "its", "our",
	"this", "that", "what", "who",
	// Common verbs
	"is", "are", "was", "were", "be", "been", "have", "has", "had",
	"do", "does", "did", "will", "would", "can", "could", "may", "might",
	"must", "shall", "should", "make", "made", "get", "got", "go", "went",
	"come", "came", "see", "saw", "know", "take", "think", "look",
	"want", "use", "find", "give", "tell", "work", "call", "try", "ask",
	"need", "feel", "seem", "leave", "put",
	// Common nouns & adjectives
	"time", "year", "day", "way", "man", "thing", "life", "hand",
	"part", "place", "case", "point", "fact", "good", "new", "first",
	"last", "long", "great", "little", "own", "other", "old", "right",
	"big", "high", "small", "large", "next", "early", "young", "few",
	"public", "bad", "same", "able",
	// Prepositions & conjunctions
	"in", "on", "at", "to", "for", "of", "with", "from", "by",
	"about", "like", "through", "over", "before", "between", "under",
	"since", "without", "and", "or", "but", "if", "than", "because",
	"as", "until", "while", "so", "though", "both", "each",
	"when", "where", "why", "how",
	// Common adverbs
	"not", "now", "just", "more", "also", "here", "there", "then",
	"only", "very", "well", "back", "still", "even", "much", "too",
	"such", "never", "again", "most", "once", "off", "away", "down", "out", "up",
	// Tech/common words
	"test", "code", "data", "file", "line", "text", "word", "number",
	"system", "program", "set", "run", "value", "name", "type", "state", "end", "start",
}

// generateLoremIpsum produces approximately targetTokens tokens of filler text.
func generateLoremIpsum(targetTokens int) string {
	var result strings.Builder
	tokens := 0

	for tokens < targetTokens {
		sentenceLength := 10 + rand.IntN(11)
		for i := 0; i < sentenceLength && tokens < targetTokens; i++ {
			word := oneTokenWords[rand.IntN(len(oneTokenWords))]
			result.WriteString(word)
			tokens++

			if i == sentenceLength-1 || tokens >= targetTokens {
				result.WriteString(". ")
			} else {
				result.WriteString(" ")
			}
		}

		// Paragraph break every ~20% of sentences
		if tokens < targetTokens && rand.Float64() < 0.2 {
			result.WriteString("\n\n")
		}
	}

	return strings.TrimSpace(result.String())
}

func registerLoremIpsumSkill() {
	skill.RegisterBundledSkill(skill.BundledSkillDefinition{
		Name:         "lorem-ipsum",
		Description:  "Generate filler text for long context testing. Specify token count as argument (e.g., /lorem-ipsum 50000). Outputs approximately the requested number of tokens.",
		ArgumentHint: "[token_count]",
		UserInvocable: true,
		GetPromptForCommand: func(args string) (string, error) {
			args = strings.TrimSpace(args)
			if args == "" {
				return generateLoremIpsum(10000), nil
			}
			parsed, err := strconv.Atoi(args)
			if err != nil || parsed <= 0 {
				return "Invalid token count. Please provide a positive number (e.g., /lorem-ipsum 10000).", nil
			}
			targetTokens := int(math.Min(float64(parsed), 500000))
			if targetTokens < parsed {
				return "Requested " + strconv.Itoa(parsed) + " tokens, but capped at 500,000 for safety.\n\n" +
					generateLoremIpsum(targetTokens), nil
			}
			return generateLoremIpsum(targetTokens), nil
		},
	})
}
