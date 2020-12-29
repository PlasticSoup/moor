package m

import (
	"os"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"gotest.tools/assert"
)

func TestUnicodeRendering(t *testing.T) {
	reader := NewReaderFromStream(strings.NewReader("åäö"))
	if err := reader._Wait(); err != nil {
		panic(err)
	}

	var answers = []Token{
		createExpectedCell('å', tcell.StyleDefault),
		createExpectedCell('ä', tcell.StyleDefault),
		createExpectedCell('ö', tcell.StyleDefault),
	}

	contents := startPaging(t, reader)
	for pos, expected := range answers {
		expected.LogDifference(t, contents[pos])
	}
}

func (expected Token) LogDifference(t *testing.T, actual tcell.SimCell) {
	if actual.Runes[0] == expected.Rune && actual.Style == expected.Style {
		return
	}

	t.Errorf("Expected '%s'/0x%x, got '%s'/0x%x",
		string(expected.Rune), expected.Style,
		string(actual.Runes[0]), actual.Style)
}

func createExpectedCell(Rune rune, Style tcell.Style) Token {
	return Token{
		Rune:  Rune,
		Style: Style,
	}
}

func TestFgColorRendering(t *testing.T) {
	reader := NewReaderFromStream(strings.NewReader(
		"\x1b[30ma\x1b[31mb\x1b[32mc\x1b[33md\x1b[34me\x1b[35mf\x1b[36mg\x1b[37mh\x1b[0mi"))
	if err := reader._Wait(); err != nil {
		panic(err)
	}

	var answers = []Token{
		createExpectedCell('a', tcell.StyleDefault.Foreground(tcell.ColorBlack)),
		createExpectedCell('b', tcell.StyleDefault.Foreground(tcell.ColorMaroon)),
		createExpectedCell('c', tcell.StyleDefault.Foreground(tcell.ColorGreen)),
		createExpectedCell('d', tcell.StyleDefault.Foreground(tcell.ColorOlive)),
		createExpectedCell('e', tcell.StyleDefault.Foreground(tcell.ColorNavy)),
		createExpectedCell('f', tcell.StyleDefault.Foreground(tcell.ColorPurple)),
		createExpectedCell('g', tcell.StyleDefault.Foreground(tcell.ColorTeal)),
		createExpectedCell('h', tcell.StyleDefault.Foreground(tcell.ColorSilver)),
		createExpectedCell('i', tcell.StyleDefault),
	}

	contents := startPaging(t, reader)
	for pos, expected := range answers {
		expected.LogDifference(t, contents[pos])
	}
}

func TestBrokenUtf8(t *testing.T) {
	// The broken UTF8 character in the middle is based on "©" = 0xc2a9
	reader := NewReaderFromStream(strings.NewReader("abc\xc2def"))
	if err := reader._Wait(); err != nil {
		panic(err)
	}

	var answers = []Token{
		createExpectedCell('a', tcell.StyleDefault),
		createExpectedCell('b', tcell.StyleDefault),
		createExpectedCell('c', tcell.StyleDefault),
		createExpectedCell('?', tcell.StyleDefault.Foreground(tcell.ColorMaroon).Background(tcell.ColorSilver)),
		createExpectedCell('d', tcell.StyleDefault),
		createExpectedCell('e', tcell.StyleDefault),
		createExpectedCell('f', tcell.StyleDefault),
	}

	contents := startPaging(t, reader)
	for pos, expected := range answers {
		expected.LogDifference(t, contents[pos])
	}
}

func startPaging(t *testing.T, reader *Reader) []tcell.SimCell {
	screen := tcell.NewSimulationScreen("UTF-8")
	pager := NewPager(reader)
	pager.showLineNumbers = false
	pager.Quit()

	var loglines strings.Builder
	pager.StartPaging(screen)
	contents, _, _ := screen.GetContents()

	if len(loglines.String()) > 0 {
		t.Logf("%s", loglines.String())
	}

	return contents
}

// assertIndexOfFirstX verifies the (zero-based) index of the first 'x'
func assertIndexOfFirstX(t *testing.T, s string, expectedIndex int) {
	reader := NewReaderFromStream(strings.NewReader(s))
	if err := reader._Wait(); err != nil {
		panic(err)
	}

	contents := startPaging(t, reader)
	for pos, cell := range contents {
		if cell.Runes[0] != 'x' {
			continue
		}

		if pos == expectedIndex {
			// Success!
			return
		}

		t.Errorf("Expected first 'x' to be at (zero-based) index %d, but was at %d: \"%s\"",
			expectedIndex, pos, strings.ReplaceAll(s, "\x09", "<TAB>"))
		return
	}

	panic("No 'x' found")
}

func TestTabHandling(t *testing.T) {
	assertIndexOfFirstX(t, "x", 0)

	assertIndexOfFirstX(t, "\x09x", 4)
	assertIndexOfFirstX(t, "\x09\x09x", 8)

	assertIndexOfFirstX(t, "J\x09x", 4)
	assertIndexOfFirstX(t, "Jo\x09x", 4)
	assertIndexOfFirstX(t, "Joh\x09x", 4)
	assertIndexOfFirstX(t, "Joha\x09x", 8)
	assertIndexOfFirstX(t, "Johan\x09x", 8)

	assertIndexOfFirstX(t, "\x09J\x09x", 8)
	assertIndexOfFirstX(t, "\x09Jo\x09x", 8)
	assertIndexOfFirstX(t, "\x09Joh\x09x", 8)
	assertIndexOfFirstX(t, "\x09Joha\x09x", 12)
	assertIndexOfFirstX(t, "\x09Johan\x09x", 12)
}

// This test assumes highlight is installed:
// http://www.andre-simon.de/zip/download.php
func TestCodeHighlighting(t *testing.T) {
	// From: https://coderwall.com/p/_fmbug/go-get-path-to-current-file
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("Getting current filename failed")
	}

	reader, err := NewReaderFromFilename(filename)
	if err != nil {
		panic(err)
	}
	if err := reader._Wait(); err != nil {
		panic(err)
	}

	var answers = []Token{
		createExpectedCell('p', tcell.StyleDefault.Foreground(tcell.ColorOlive)),
		createExpectedCell('a', tcell.StyleDefault.Foreground(tcell.ColorOlive)),
		createExpectedCell('c', tcell.StyleDefault.Foreground(tcell.ColorOlive)),
		createExpectedCell('k', tcell.StyleDefault.Foreground(tcell.ColorOlive)),
		createExpectedCell('a', tcell.StyleDefault.Foreground(tcell.ColorOlive)),
		createExpectedCell('g', tcell.StyleDefault.Foreground(tcell.ColorOlive)),
		createExpectedCell('e', tcell.StyleDefault.Foreground(tcell.ColorOlive)),
		createExpectedCell(' ', tcell.StyleDefault),
		createExpectedCell('m', tcell.StyleDefault),
	}

	contents := startPaging(t, reader)
	for pos, expected := range answers {
		expected.LogDifference(t, contents[pos])
	}
}

func testManPageFormatting(t *testing.T, input string, expected Token) {
	reader := NewReaderFromStream(strings.NewReader(input))
	if err := reader._Wait(); err != nil {
		panic(err)
	}

	// Without these three lines the man page tests will fail if either of these
	// environment variables are set when the tests are run.
	os.Setenv("LESS_TERMCAP_md", "")
	os.Setenv("LESS_TERMCAP_us", "")
	resetManPageFormatForTesting()

	contents := startPaging(t, reader)
	expected.LogDifference(t, contents[0])
	assert.Equal(t, contents[1].Runes[0], ' ')
}

func TestManPageFormatting(t *testing.T) {
	testManPageFormatting(t, "N\x08N", createExpectedCell('N', tcell.StyleDefault.Bold(true)))
	testManPageFormatting(t, "_\x08x", createExpectedCell('x', tcell.StyleDefault.Underline(true)))

	// Corner cases
	testManPageFormatting(t, "\x08", createExpectedCell('<', tcell.StyleDefault.Foreground(tcell.ColorMaroon).Background(tcell.ColorSilver)))

	// FIXME: Test two consecutive backspaces

	// FIXME: Test backspace between two uncombinable characters
}

func TestToPattern(t *testing.T) {
	assert.Assert(t, ToPattern("") == nil)

	// Test regexp matching
	assert.Assert(t, ToPattern("G.*S").MatchString("GRIIIS"))
	assert.Assert(t, !ToPattern("G.*S").MatchString("gRIIIS"))

	// Test case insensitive regexp matching
	assert.Assert(t, ToPattern("g.*s").MatchString("GRIIIS"))
	assert.Assert(t, ToPattern("g.*s").MatchString("gRIIIS"))

	// Test non-regexp matching
	assert.Assert(t, ToPattern(")G").MatchString(")G"))
	assert.Assert(t, !ToPattern(")G").MatchString(")g"))

	// Test case insensitive non-regexp matching
	assert.Assert(t, ToPattern(")g").MatchString(")G"))
	assert.Assert(t, ToPattern(")g").MatchString(")g"))
}

func assertTokenRangesEqual(t *testing.T, actual []Token, expected []Token) {
	if len(actual) != len(expected) {
		t.Errorf("String lengths mismatch; expected %d but got %d",
			len(expected), len(actual))
	}

	for pos, expectedToken := range expected {
		if pos >= len(expected) || pos >= len(actual) {
			break
		}

		actualToken := actual[pos]
		if actualToken.Rune == expectedToken.Rune && actualToken.Style == expectedToken.Style {
			// Actual == Expected, keep checking
			continue
		}

		t.Errorf("At (0-based) position %d: Expected '%s'/0x%x, got '%s'/0x%x",
			pos,
			string(expectedToken.Rune), expectedToken.Style,
			string(actualToken.Rune), actualToken.Style)
	}
}

func TestCreateScreenLineBase(t *testing.T) {
	line := createScreenLine(0, 3, "", nil)
	assert.Assert(t, len(line) == 0)
}

func TestCreateScreenLineOverflowRight(t *testing.T) {
	line := createScreenLine(0, 3, "012345", nil)
	assertTokenRangesEqual(t, line, []Token{
		createExpectedCell('0', tcell.StyleDefault),
		createExpectedCell('1', tcell.StyleDefault),
		createExpectedCell('>', tcell.StyleDefault.Reverse(true)),
	})
}

func TestCreateScreenLineUnderflowLeft(t *testing.T) {
	line := createScreenLine(1, 3, "012", nil)
	assertTokenRangesEqual(t, line, []Token{
		createExpectedCell('<', tcell.StyleDefault.Reverse(true)),
		createExpectedCell('1', tcell.StyleDefault),
		createExpectedCell('2', tcell.StyleDefault),
	})
}

func TestCreateScreenLineSearchHit(t *testing.T) {
	pattern, err := regexp.Compile("b")
	if err != nil {
		panic(err)
	}

	line := createScreenLine(0, 3, "abc", pattern)
	assertTokenRangesEqual(t, line, []Token{
		createExpectedCell('a', tcell.StyleDefault),
		createExpectedCell('b', tcell.StyleDefault.Reverse(true)),
		createExpectedCell('c', tcell.StyleDefault),
	})
}

func TestCreateScreenLineUtf8SearchHit(t *testing.T) {
	pattern, err := regexp.Compile("ä")
	if err != nil {
		panic(err)
	}

	line := createScreenLine(0, 3, "åäö", pattern)
	assertTokenRangesEqual(t, line, []Token{
		createExpectedCell('å', tcell.StyleDefault),
		createExpectedCell('ä', tcell.StyleDefault.Reverse(true)),
		createExpectedCell('ö', tcell.StyleDefault),
	})
}

func TestCreateScreenLineScrolledUtf8SearchHit(t *testing.T) {
	pattern := regexp.MustCompile("ä")

	line := createScreenLine(1, 4, "ååäö", pattern)

	assertTokenRangesEqual(t, line, []Token{
		createExpectedCell('<', tcell.StyleDefault.Reverse(true)),
		createExpectedCell('å', tcell.StyleDefault),
		createExpectedCell('ä', tcell.StyleDefault.Reverse(true)),
		createExpectedCell('ö', tcell.StyleDefault),
	})
}

func TestCreateScreenLineScrolled2Utf8SearchHit(t *testing.T) {
	pattern := regexp.MustCompile("ä")

	line := createScreenLine(2, 4, "åååäö", pattern)

	assertTokenRangesEqual(t, line, []Token{
		createExpectedCell('<', tcell.StyleDefault.Reverse(true)),
		createExpectedCell('å', tcell.StyleDefault),
		createExpectedCell('ä', tcell.StyleDefault.Reverse(true)),
		createExpectedCell('ö', tcell.StyleDefault),
	})
}

func TestFindFirstLineOneBasedSimple(t *testing.T) {
	reader := NewReaderFromStream(strings.NewReader("AB"))
	pager := NewPager(reader)

	// Wait for reader to finish reading
	<-reader.done

	pager.searchPattern = ToPattern("AB")

	hitLine := pager._FindFirstHitLineOneBased(1, false)
	assert.Check(t, hitLine != nil)
	assert.Check(t, *hitLine == 1)
}

func TestFindFirstLineOneBasedAnsi(t *testing.T) {
	reader := NewReaderFromStream(strings.NewReader("A\x1b[30mB"))
	pager := NewPager(reader)

	// Wait for reader to finish reading
	<-reader.done

	pager.searchPattern = ToPattern("AB")

	hitLine := pager._FindFirstHitLineOneBased(1, false)
	assert.Check(t, hitLine != nil)
	assert.Check(t, *hitLine == 1)
}
