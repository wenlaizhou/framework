package framework

import (
	"github.com/russross/blackfriday"
	"strings"
)

const (
	commonExtensions = 0 |
		blackfriday.EXTENSION_NO_INTRA_EMPHASIS |
		blackfriday.EXTENSION_TABLES |
		blackfriday.EXTENSION_FENCED_CODE |
		blackfriday.EXTENSION_AUTOLINK |
		blackfriday.EXTENSION_STRIKETHROUGH |
		blackfriday.EXTENSION_SPACE_HEADERS |
		blackfriday.EXTENSION_HEADER_IDS |
		blackfriday.EXTENSION_BACKSLASH_LINE_BREAK |
		blackfriday.EXTENSION_DEFINITION_LISTS
	commonHtmlFlags = 0 |
		blackfriday.HTML_USE_XHTML |
		blackfriday.HTML_USE_SMARTYPANTS |
		blackfriday.HTML_SMARTYPANTS_FRACTIONS |
		blackfriday.HTML_SMARTYPANTS_DASHES |
		blackfriday.HTML_SMARTYPANTS_LATEX_DASHES
)

func MarkDown(data []byte) string {
	renderer := blackfriday.HtmlRenderer(commonHtmlFlags|blackfriday.HTML_TOC, "", "")
	res := string(blackfriday.MarkdownOptions(data, renderer, blackfriday.Options{
		Extensions: commonExtensions}))
	return strings.Replace(res, "[TOC]", "", 1)
}
