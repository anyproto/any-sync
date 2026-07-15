package pubsub

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateTopic(t *testing.T) {
	valid := []string{
		"presence",
		"chat/abc/typing",
		"acc/online/A5xyz",
		strings.Repeat("a/", 15) + "a", // 16 segments
	}
	for _, topic := range valid {
		require.NoError(t, ValidateTopic(topic), topic)
	}
	invalid := []string{
		"",
		"/presence",
		"presence/",
		"a//b",
		"chat/*/typing",
		"chat/>",
		"chat/ty*",
		"chat/ty>pe",
		strings.Repeat("a/", 16) + "a", // 17 segments
		strings.Repeat("x", 257),
	}
	for _, topic := range invalid {
		require.Error(t, ValidateTopic(topic), topic)
	}
}

func TestValidatePattern(t *testing.T) {
	valid := []string{
		"presence",
		"chat/*/typing",
		"chat/>",
		">",
		"*",
		"acc/online/*",
		"a/*/*/b",
	}
	for _, p := range valid {
		require.NoError(t, ValidatePattern(p), p)
	}
	invalid := []string{
		"",
		"/chat/*",
		"chat/>/typing", // '>' not in tail position
		"chat/ty*",      // wildcard not a whole segment
		"chat/*>",
		"a//>",
	}
	for _, p := range invalid {
		require.Error(t, ValidatePattern(p), p)
	}
}

func TestTopicOwner(t *testing.T) {
	require.Equal(t, "A5xyz", TopicOwner("acc/online/A5xyz"))
	require.Equal(t, "A5xyz", TopicOwner("acc/cursor/obj1/A5xyz"))
	require.Equal(t, "", TopicOwner("presence"))
	require.Equal(t, "", TopicOwner("chat/abc/typing"))
	// bare "acc" has no owner segment
	require.Equal(t, "", TopicOwner("acc"))
	// non-acc first segment
	require.Equal(t, "", TopicOwner("accounts/online/A5xyz"))
}
