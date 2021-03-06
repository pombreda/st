package wikidump

import (
	"bytes"
	"golang.org/x/text/unicode/norm"
	"html"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	special  = regexp.MustCompile(`{{|{\||\|}|}}|<[a-z][a-z0-9 "=]*/?>|</[a-z]+>`)
	starttag = regexp.MustCompile(`<[a-z].*>`)
	endtag   = regexp.MustCompile(`</[a-z]+>`)
)

// Get rid of tables, template calls, quasi-XML. Throws away their content.
//
// Assumes tables, templates and tags are properly nested, except for spurious
// end-of-{table,template,element} tags, which are ignored.
func Cleanup(s string) string {
	var depth int
	output := bytes.NewBuffer(make([]byte, 0, len(s)))

	for {
		next := special.FindStringIndex(s)
		if next == nil {
			if depth == 0 {
				output.WriteString(s)
			}
			break
		}
		i, j := next[0], next[1]

		if depth == 0 {
			output.WriteString(s[:i])
		}

		tag := s[i:j]
		switch {
		case tag == "{{":
			depth++
		case tag == "{|":
			depth++
		case starttag.MatchString(tag):
			depth++
		case tag == "}}":
			fallthrough
		case tag == "|}":
			fallthrough
		case endtag.MatchString(tag):
			if depth > 0 {
				depth--
			}
		}

		s = s[j:]
	}
	return norm.NFC.String(html.UnescapeString(output.String()))
}

// A link to the article Target with anchor text Anchor.
type Link struct {
	Anchor, Target string
}

var (
	linkRE     = regexp.MustCompile(`(\w*)\[\[([^]]+)\]\](\w*)`)
	whitespace = regexp.MustCompile(`[\s_]+`)
)

func normSpace(s string) string {
	s = whitespace.ReplaceAllLiteralString(s, " ")
	return strings.TrimSpace(s)
}

// Extract all the wikilinks from s. Returns a frequency table.
func ExtractLinks(s string) map[Link]int {
	freq := make(map[Link]int)

	for _, candidate := range linkRE.FindAllStringSubmatch(s, -1) {
		before, l, after := candidate[1], candidate[2], candidate[3]

		var target, anchor string
		if pipe := strings.IndexByte(l, '|'); pipe != -1 {
			target, anchor = l[:pipe], l[pipe+1:]
		} else {
			target = l
			anchor = l
		}

		// If the anchor contains a colon, assume it's a file or category link.
		// XXX Maybe skip matches for `:\s`? Proper solution would parse the
		// dump to find non-main namespace prefixes.
		if strings.IndexByte(target, ':') != -1 {
			continue
		}

		// Remove section links.
		if hash := strings.IndexByte(target, '#'); hash == 0 {
			continue
		} else if hash != -1 {
			target = target[:hash]
		}

		// Normalize to the format used in <redirect> elements:
		// uppercase first character, spaces instead of underscores.
		target = normSpace(target)
		first, size := utf8.DecodeRuneInString(target)
		// XXX Upper case or title case? Should look up the difference...
		if unicode.IsLower(first) {
			target = string(unicode.ToUpper(first)) + target[size:]
		}

		anchor = before + anchor + after
		freq[Link{anchor, target}]++
	}
	return freq
}
