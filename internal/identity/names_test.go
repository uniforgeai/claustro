package identity

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

var namePattern = regexp.MustCompile(`^[a-z]+_[a-z]+$`)

func TestRandomName_Format(t *testing.T) {
	name := RandomName()
	assert.Regexp(t, namePattern, name, "RandomName() should match ^[a-z]+_[a-z]+$")
}

func TestRandomName_RepeatedCalls(t *testing.T) {
	for i := 0; i < 10; i++ {
		name := RandomName()
		assert.Regexp(t, namePattern, name, "RandomName() call %d should match ^[a-z]+_[a-z]+$", i)
	}
}

func TestRandomName_UsesWordLists(t *testing.T) {
	// Verify the name is composed of a known adjective and noun
	name := RandomName()
	assert.Regexp(t, namePattern, name)

	// Split on underscore and verify both parts are in their respective word lists
	var adj, noun string
	_, err := sscanf(name, &adj, &noun)
	assert.NoError(t, err)
	assert.Contains(t, adjectives, adj)
	assert.Contains(t, nouns, noun)
}

// sscanf is a minimal helper to split "adj_noun" into two parts.
func sscanf(name string, adj, noun *string) (int, error) {
	for i, c := range name {
		if c == '_' {
			*adj = name[:i]
			*noun = name[i+1:]
			return 2, nil
		}
	}
	return 0, nil
}
