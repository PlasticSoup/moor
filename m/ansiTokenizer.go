package m

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/gdamore/tcell/v2"
)

const _TabSize = 4

var manPageBold = tcell.StyleDefault.Bold(true)
var manPageUnderline = tcell.StyleDefault.Underline(true)

// Token is a rune with a style to be written to a cell on screen
type Token struct {
	Rune  rune
	Style tcell.Style
}

// SetManPageFormatFromEnv parses LESS_TERMCAP_xx environment variables and
// adapts the moar output accordingly.
func SetManPageFormatFromEnv() {
	// Requested here: https://github.com/walles/moar/issues/14

	lessTermcapMd := os.Getenv("LESS_TERMCAP_md")
	if lessTermcapMd != "" {
		manPageBold = termcapToStyle(lessTermcapMd)
	}

	lessTermcapUs := os.Getenv("LESS_TERMCAP_us")
	if lessTermcapUs != "" {
		manPageUnderline = termcapToStyle(lessTermcapUs)
	}
}

// Used from tests
func resetManPageFormatForTesting() {
	manPageBold = tcell.StyleDefault.Bold(true)
	manPageUnderline = tcell.StyleDefault.Underline(true)
}

func termcapToStyle(termcap string) tcell.Style {
	// Add a character to be sure we have one to take the format from
	tokens, _ := TokensFromString(termcap + "x")
	return tokens[len(tokens)-1].Style
}

// TokensFromString turns a (formatted) string into a series of tokens,
// and an unformatted string
func TokensFromString(s string) ([]Token, *string) {
	var tokens []Token

	styleBrokenUtf8 := tcell.StyleDefault.Background(tcell.ColorSilver).Foreground(tcell.ColorMaroon)

	for _, styledString := range styledStringsFromString(s) {
		for _, token := range tokensFromStyledString(styledString) {
			switch token.Rune {

			case '\x09': // TAB
				for {
					tokens = append(tokens, Token{
						Rune:  ' ',
						Style: styledString.Style,
					})

					if (len(tokens))%_TabSize == 0 {
						// We arrived at the next tab stop
						break
					}
				}

			case '�': // Go's broken-UTF8 marker
				tokens = append(tokens, Token{
					Rune:  '?',
					Style: styleBrokenUtf8,
				})

			case '\x08': // Backspace
				tokens = append(tokens, Token{
					Rune:  '<',
					Style: styleBrokenUtf8,
				})

			default:
				tokens = append(tokens, token)
			}
		}
	}

	var stringBuilder strings.Builder
	stringBuilder.Grow(len(tokens))
	for _, token := range tokens {
		stringBuilder.WriteRune(token.Rune)
	}
	plainString := stringBuilder.String()
	return tokens, &plainString
}

// Consume 'x<x', where '<' is backspace and the result is a bold 'x'
func consumeBold(runes []rune, index int) (int, *Token) {
	if index+2 >= len(runes) {
		// Not enough runes left for a bold
		return index, nil
	}

	if runes[index+1] != '\b' {
		// No backspace in the middle, never mind
		return index, nil
	}

	if runes[index] != runes[index+2] {
		// First and last rune not the same, never mind
		return index, nil
	}

	// We have a match!
	return index + 3, &Token{
		Rune:  runes[index],
		Style: manPageBold,
	}
}

// Consume '_<x', where '<' is backspace and the result is an underlined 'x'
func consumeUnderline(runes []rune, index int) (int, *Token) {
	if index+2 >= len(runes) {
		// Not enough runes left for a underline
		return index, nil
	}

	if runes[index+1] != '\b' {
		// No backspace in the middle, never mind
		return index, nil
	}

	if runes[index] != '_' {
		// No underline, never mind
		return index, nil
	}

	// We have a match!
	return index + 3, &Token{
		Rune:  runes[index+2],
		Style: manPageUnderline,
	}
}

// Consume '+<+<o<o' / '+<o', where '<' is backspace and the result is a unicode bullet.
//
// Used on man pages, try "man printf" on macOS for one example.
func consumeBullet(runes []rune, index int) (int, *Token) {
	patterns := []string{"+\bo", "+\b+\bo\bo"}
	for _, pattern := range patterns {
		if index+len(pattern) > len(runes) {
			// Not enough runes left for a bullet
			continue
		}

		mismatch := false
		for delta, patternChar := range pattern {
			if rune(patternChar) != runes[index+delta] {
				// Bullet pattern mismatch, never mind
				mismatch = true
			}
		}
		if mismatch {
			continue
		}

		// We have a match!
		return index + len(pattern), &Token{
			Rune:  '•', // Unicode bullet point
			Style: tcell.StyleDefault,
		}
	}

	return index, nil
}

func tokensFromStyledString(styledString _StyledString) []Token {
	runes := []rune(styledString.String)
	tokens := make([]Token, 0, len(runes))

	for index := 0; index < len(runes); index++ {
		nextIndex, token := consumeBullet(runes, index)
		if nextIndex != index {
			tokens = append(tokens, *token)
			index = nextIndex - 1
			continue
		}

		nextIndex, token = consumeBold(runes, index)
		if nextIndex != index {
			tokens = append(tokens, *token)
			index = nextIndex - 1
			continue
		}

		nextIndex, token = consumeUnderline(runes, index)
		if nextIndex != index {
			tokens = append(tokens, *token)
			index = nextIndex - 1
			continue
		}

		tokens = append(tokens, Token{
			Rune:  runes[index],
			Style: styledString.Style,
		})
	}

	return tokens
}

type _StyledString struct {
	String string
	Style  tcell.Style
}

func styledStringsFromString(s string) []_StyledString {
	// This function was inspired by the
	// https://golang.org/pkg/regexp/#Regexp.Split source code

	pattern := regexp.MustCompile("\x1b\\[([0-9;]*m)")

	matches := pattern.FindAllStringIndex(s, -1)
	styledStrings := make([]_StyledString, 0, len(matches)+1)

	style := tcell.StyleDefault

	beg := 0
	end := 0
	for _, match := range matches {
		end = match[0]

		if end > beg {
			// Found non-zero length string
			styledStrings = append(styledStrings, _StyledString{
				String: s[beg:end],
				Style:  style,
			})
		}

		matchedPart := s[match[0]:match[1]]
		style = updateStyle(style, matchedPart)

		beg = match[1]
	}

	if end != len(s) {
		styledStrings = append(styledStrings, _StyledString{
			String: s[beg:],
			Style:  style,
		})
	}

	return styledStrings
}

// updateStyle parses a string of the form "ESC[33m" into changes to style
func updateStyle(style tcell.Style, escapeSequence string) tcell.Style {
	numbers := strings.Split(escapeSequence[2:len(escapeSequence)-1], ";")
	index := 0
	for index < len(numbers) {
		number := numbers[index]
		index++
		switch strings.TrimLeft(number, "0") {
		case "":
			style = tcell.StyleDefault

		case "1":
			style = style.Bold(true)

		case "2":
			style = style.Dim(true)

		case "3":
			style = style.Italic(true)

		case "4":
			style = style.Underline(true)

		case "7":
			style = style.Reverse(true)

		case "22":
			style = style.Bold(false).Dim(false)

		case "23":
			style = style.Italic(false)

		case "24":
			style = style.Underline(false)

		case "27":
			style = style.Reverse(false)

		// Foreground colors, https://pkg.go.dev/github.com/gdamore/tcell#Color
		case "30":
			style = style.Foreground(tcell.ColorBlack)
		case "31":
			style = style.Foreground(tcell.ColorMaroon)
		case "32":
			style = style.Foreground(tcell.ColorGreen)
		case "33":
			style = style.Foreground(tcell.ColorOlive)
		case "34":
			style = style.Foreground(tcell.ColorNavy)
		case "35":
			style = style.Foreground(tcell.ColorPurple)
		case "36":
			style = style.Foreground(tcell.ColorTeal)
		case "37":
			style = style.Foreground(tcell.ColorSilver)
		case "38":
			var err error = nil
			var color *tcell.Color
			index, color, err = consumeCompositeColor(numbers, index-1)
			if err != nil {
				log.Warnf("Foreground: %s", err.Error())
				return style
			}
			style = style.Foreground(*color)
		case "39":
			style = style.Foreground(tcell.ColorDefault)

		// Background colors, see https://pkg.go.dev/github.com/gdamore/tcell#Color
		case "40":
			style = style.Background(tcell.ColorBlack)
		case "41":
			style = style.Background(tcell.ColorMaroon)
		case "42":
			style = style.Background(tcell.ColorGreen)
		case "43":
			style = style.Background(tcell.ColorOlive)
		case "44":
			style = style.Background(tcell.ColorNavy)
		case "45":
			style = style.Background(tcell.ColorPurple)
		case "46":
			style = style.Background(tcell.ColorTeal)
		case "47":
			style = style.Background(tcell.ColorSilver)
		case "48":
			var err error = nil
			var color *tcell.Color
			index, color, err = consumeCompositeColor(numbers, index-1)
			if err != nil {
				log.Warnf("Background: %s", err.Error())
				return style
			}
			style = style.Background(*color)
		case "49":
			style = style.Background(tcell.ColorDefault)

		// Bright foreground colors: see https://pkg.go.dev/github.com/gdamore/tcell#Color
		//
		// After testing vs less and cat on iTerm2 3.3.9 / macOS Catalina
		// 10.15.4 that's how they seem to handle this, tested with:
		// * TERM=xterm-256color
		// * TERM=screen-256color
		case "90":
			style = style.Foreground(tcell.ColorGray)
		case "91":
			style = style.Foreground(tcell.ColorRed)
		case "92":
			style = style.Foreground(tcell.ColorLime)
		case "93":
			style = style.Foreground(tcell.ColorYellow)
		case "94":
			style = style.Foreground(tcell.ColorBlue)
		case "95":
			style = style.Foreground(tcell.ColorFuchsia)
		case "96":
			style = style.Foreground(tcell.ColorAqua)
		case "97":
			style = style.Foreground(tcell.ColorWhite)

		default:
			log.Warnf("Unrecognized ANSI SGR code <%s>", number)
		}
	}

	return style
}

// numbers is a list of numbers from a ANSI SGR string
// index points to either 38 or 48 in that string
//
// This method will return:
// * The first index in the string that this function did not consume
// * A color value that can be applied to a style
func consumeCompositeColor(numbers []string, index int) (int, *tcell.Color, error) {
	baseIndex := index
	if numbers[index] != "38" && numbers[index] != "48" {
		err := fmt.Errorf(
			"Unknown start of color sequence <%s>, expected 38 (foreground) or 48 (background): <CSI %sm>",
			numbers[index],
			strings.Join(numbers[baseIndex:], ";"))
		return -1, nil, err
	}

	index++
	if index >= len(numbers) {
		err := fmt.Errorf(
			"Incomplete color sequence: <CSI %sm>",
			strings.Join(numbers[baseIndex:], ";"))
		return -1, nil, err
	}

	if numbers[index] == "5" {
		// Handle 8 bit color
		index++
		if index >= len(numbers) {
			err := fmt.Errorf(
				"Incomplete 8 bit color sequence: <CSI %sm>",
				strings.Join(numbers[baseIndex:], ";"))
			return -1, nil, err
		}

		colorNumber, err := strconv.Atoi(numbers[index])
		if err != nil {
			return -1, nil, err
		}

		// Workaround for https://github.com/gdamore/tcell/issues/404
		colorValue := tcell.Color(int64(tcell.Color16) - 16 + int64(colorNumber))
		return index + 1, &colorValue, nil
	}

	if numbers[index] == "2" {
		// Handle 24 bit color
		rIndex := index + 1
		gIndex := index + 2
		bIndex := index + 3
		if bIndex >= len(numbers) {
			err := fmt.Errorf(
				"Incomplete 24 bit color sequence, expected N8;2;R;G;Bm: <CSI %sm>",
				strings.Join(numbers[baseIndex:], ";"))
			return -1, nil, err
		}

		rValueX, err := strconv.ParseInt(numbers[rIndex], 10, 32)
		if err != nil {
			return -1, nil, err
		}
		rValue := int32(rValueX)

		gValueX, err := strconv.Atoi(numbers[gIndex])
		if err != nil {
			return -1, nil, err
		}
		gValue := int32(gValueX)

		bValueX, err := strconv.Atoi(numbers[bIndex])
		if err != nil {
			return -1, nil, err
		}
		bValue := int32(bValueX)

		colorValue := tcell.NewRGBColor(rValue, gValue, bValue)
		return bIndex + 1, &colorValue, nil
	}

	err := fmt.Errorf(
		"Unknown color type <%s>, expected 5 (8 bit color) or 2 (24 bit color): <CSI %sm>",
		numbers[index],
		strings.Join(numbers[baseIndex:], ";"))
	return -1, nil, err
}
