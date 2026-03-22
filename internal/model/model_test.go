package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNote_SanitizedFileName_Normal(t *testing.T) {
	n := &Note{Name: "My Note Title"}
	assert.Equal(t, "My Note Title", n.SanitizedFileName())
}

func TestNote_SanitizedFileName_SpecialChars(t *testing.T) {
	n := &Note{Name: `My/Special:Note*Title?"yes"<no>|pipe`}
	assert.Equal(t, "My-Special-Note-Title--yes--no--pipe", n.SanitizedFileName())
}

func TestNote_SanitizedFileName_Empty(t *testing.T) {
	n := &Note{Name: ""}
	assert.Equal(t, "untitled", n.SanitizedFileName())
}

func TestNote_SanitizedFileName_OnlySpecialChars(t *testing.T) {
	n := &Note{Name: "///"}
	assert.Equal(t, "---", n.SanitizedFileName())
}

func TestNote_SanitizedFileName_TrailingDotsAndSpaces(t *testing.T) {
	n := &Note{Name: "Note...  "}
	assert.Equal(t, "Note", n.SanitizedFileName())
}

func TestNote_SanitizedFileName_AllDotsAndSpaces(t *testing.T) {
	n := &Note{Name: "... "}
	assert.Equal(t, "untitled", n.SanitizedFileName())
}

func TestNote_SanitizedFileName_LongName(t *testing.T) {
	long := ""
	for i := 0; i < 250; i++ {
		long += "a"
	}
	n := &Note{Name: long}
	result := n.SanitizedFileName()
	assert.Len(t, result, 200)
}

func TestNote_SanitizedFileName_Backslash(t *testing.T) {
	n := &Note{Name: `path\to\note`}
	assert.Equal(t, "path-to-note", n.SanitizedFileName())
}
