package awsdag

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
)

// ErrNoChoices is returned when there is nothing to choose from, which for an
// account or a permission set means nothing has been assigned.
var ErrNoChoices = errors.New("nothing to choose from")

// Choose asks which of items to use, and returns it.
//
// A single item is taken without asking: there is no choice to make, and a
// prompt with one answer is just something else to press return on. Anything
// unreadable is asked again rather than treated as an answer, because the
// question is which set of credentials to mint and guessing is not an option.
func Choose[T any](in io.Reader, out io.Writer, prompt string, items []T, label func(T) string) (T, error) {
	var zero T

	switch len(items) {
	case 0:
		return zero, fmt.Errorf("%s: %w", prompt, ErrNoChoices)
	case 1:
		return items[0], nil
	}

	for i, item := range items {
		if _, err := fmt.Fprintf(out, "%3d. %s\n", i+1, label(item)); err != nil {
			return zero, err
		}
	}

	scanner := bufio.NewScanner(in)

	for {
		if _, err := fmt.Fprintf(out, "%s [1-%d]: ", prompt, len(items)); err != nil {
			return zero, err
		}

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return zero, err
			}

			return zero, errors.New("no selection was made")
		}

		n, err := strconv.Atoi(scanner.Text())

		if err != nil || n < 1 || n > len(items) {
			fmt.Fprintf(out, "Enter a number between 1 and %d.\n", len(items)) //nolint:errcheck

			continue
		}

		return items[n-1], nil
	}
}
