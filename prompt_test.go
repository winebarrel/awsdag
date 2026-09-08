package awsdag_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/winebarrel/awsdag"
)

// identity labels a string as itself.
func identity(s string) string { return s }

func TestChoose(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	out := &bytes.Buffer{}

	choice, err := awsdag.Choose(strings.NewReader("2\n"), out, "Role", []string{"a", "b", "c"}, identity)

	require.NoError(err)
	assert.Equal("b", choice)
	assert.Contains(out.String(), "  1. a")
	assert.Contains(out.String(), "  2. b")
	assert.Contains(out.String(), "Role [1-3]: ")
}

func TestChoose_OnlyOne(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	out := &bytes.Buffer{}

	// Nothing is read and nothing is printed: there is no choice to make, and
	// a prompt with one answer is only something else to press return on.
	choice, err := awsdag.Choose(strings.NewReader(""), out, "Role", []string{"a"}, identity)

	require.NoError(err)
	assert.Equal("a", choice)
	assert.Empty(out.String())
}

func TestChoose_None(t *testing.T) {
	_, err := awsdag.Choose(strings.NewReader(""), &bytes.Buffer{}, "Account", []string{}, identity)

	assert.ErrorIs(t, err, awsdag.ErrNoChoices)
	assert.ErrorContains(t, err, "Account")
}

func TestChoose_AsksAgain(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	out := &bytes.Buffer{}

	// Which credentials to mint is not a question to answer by guessing, so
	// anything unreadable is asked again rather than rounded to something.
	choice, err := awsdag.Choose(strings.NewReader("x\n0\n9\n3\n"), out, "Role", []string{"a", "b", "c"}, identity)

	require.NoError(err)
	assert.Equal("c", choice)
	assert.Equal(3, strings.Count(out.String(), "Enter a number between 1 and 3."))
}

func TestChoose_NoAnswer(t *testing.T) {
	_, err := awsdag.Choose(strings.NewReader(""), &bytes.Buffer{}, "Role", []string{"a", "b"}, identity)

	assert.ErrorContains(t, err, "no selection was made")
}

func TestChoose_Accounts(t *testing.T) {
	out := &bytes.Buffer{}

	accounts := []awsdag.Account{
		{ID: "111122223333", Name: "dev"},
		{ID: "444455556666", Name: "prod"},
	}

	choice, err := awsdag.Choose(strings.NewReader("2\n"), out, "Account", accounts, awsdag.Account.String)

	require.NoError(t, err)
	assert.Equal(t, accounts[1], choice)
	assert.Contains(t, out.String(), "  2. prod (444455556666)")
}
