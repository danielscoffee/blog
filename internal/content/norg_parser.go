package content

import (
	"bytes"
	"fmt"
	"html"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

type norgNodeKind int

const (
	norgHeading norgNodeKind = iota
	norgParagraph
	norgUL
	norgOL
	norgTaskList
	norgCode
	norgTable
	norgQuote
	norgDefinitionList
)

type norgNode struct {
	kind            norgNodeKind
	level           int
	text            string
	items           []string
	taskItems       []norgTaskItem
	lang            string
	code            string
	tableHTML       string
	definitionItems []norgDefinitionItem
	attrs           map[string]string
}

type norgTaskItem struct {
	state string
	text  string
}

type norgDefinitionItem struct {
	term string
	body string
}

func parseNorg(raw string) (frontMatter, string, string, error) {
	meta, bodyLines, err := splitNorgFrontMatter(raw)
	if err != nil {
		return frontMatter{}, "", "", err
	}

	nodes, err := parseNorgBlocks(bodyLines)
	if err != nil {
		return frontMatter{}, "", "", err
	}

	body := strings.TrimSpace(strings.Join(bodyLines, "\n"))
	htmlBody, err := renderNorgHTML(nodes)
	if err != nil {
		return frontMatter{}, "", "", err
	}
	return meta, body, htmlBody, nil
}

func splitNorgFrontMatter(raw string) (frontMatter, []string, error) {
	var meta frontMatter

	trimmed := strings.TrimSpace(raw)
	lines := strings.Split(trimmed, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "@document.meta" {
		return meta, nil, fmt.Errorf("missing frontmatter delimiter")
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "@end" {
			end = i
			break
		}
	}
	if end == -1 {
		return meta, nil, fmt.Errorf("invalid frontmatter structure")
	}

	parsed, err := parseNeorgMeta(lines[1:end])
	if err != nil {
		return meta, nil, err
	}
	if err := validateFrontMatter(parsed); err != nil {
		return meta, nil, err
	}

	return parsed, lines[end+1:], nil
}

func parseNeorgMeta(lines []string) (frontMatter, error) {
	meta := frontMatter{}
	openListKey := ""

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" {
			continue
		}

		if openListKey != "" {
			if strings.HasPrefix(trimmed, "-") {
				item := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
				if openListKey == "tags" && item != "" {
					meta.Tags = append(meta.Tags, stripQuotes(item))
					continue
				}
			}
			openListKey = ""
		}

		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			return meta, fmt.Errorf("invalid neorg metadata line %q", trimmed)
		}

		k := strings.ToLower(strings.TrimSpace(key))
		v := strings.TrimSpace(value)
		if v == "" {
			openListKey = k
			continue
		}

		switch k {
		case "title":
			meta.Title = stripQuotes(v)
		case "slug":
			meta.Slug = stripQuotes(v)
		case "date":
			meta.Date = stripQuotes(v)
		case "summary":
			meta.Summary = stripQuotes(v)
		case "draft":
			draft, err := strconv.ParseBool(stripQuotes(v))
			if err != nil {
				return meta, fmt.Errorf("invalid draft value %q", v)
			}
			meta.Draft = draft
		case "tags":
			meta.Tags = append(meta.Tags, parseTagValues(v)...)
		}
	}

	return meta, nil
}

func parseTagValues(raw string) []string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return nil
	}

	if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
		v = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(v, "["), "]"))
	}

	parts := strings.Split(v, ",")
	if len(parts) == 1 {
		item := stripQuotes(strings.TrimSpace(parts[0]))
		if item == "" {
			return nil
		}
		return []string{item}
	}

	out := make([]string, 0, len(parts))
	for _, p := range parts {
		item := stripQuotes(strings.TrimSpace(p))
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func stripQuotes(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 {
		if (strings.HasPrefix(v, "\"") && strings.HasSuffix(v, "\"")) ||
			(strings.HasPrefix(v, "'") && strings.HasSuffix(v, "'")) {
			return v[1 : len(v)-1]
		}
	}
	return v
}

func parseNorgBlocks(lines []string) ([]norgNode, error) {
	nodes := make([]norgNode, 0)
	pendingAttrs := map[string]string{}
	for i := 0; i < len(lines); {
		line := strings.TrimRight(lines[i], "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			i++
			continue
		}

		if key, value, ok := parseCarryoverMeta(trimmed); ok {
			pendingAttrs[key] = value
			i++
			continue
		}

		attrs := takeAttrs(pendingAttrs)

		if lang, ok := parseAtCodeStart(trimmed); ok {
			i++
			codeLines := make([]string, 0)
			closed := false
			for i < len(lines) {
				current := strings.TrimRight(lines[i], "\r")
				if strings.TrimSpace(current) == "@end" {
					closed = true
					i++
					break
				}
				codeLines = append(codeLines, current)
				i++
			}
			if !closed {
				return nil, fmt.Errorf("unclosed @code block")
			}
			nodes = append(nodes, norgNode{kind: norgCode, lang: lang, code: strings.Join(codeLines, "\n"), attrs: attrs})
			continue
		}

		if isFenceStart(trimmed) {
			lang := strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
			i++
			codeLines := make([]string, 0)
			closed := false
			for i < len(lines) {
				current := strings.TrimRight(lines[i], "\r")
				if strings.TrimSpace(current) == "```" {
					closed = true
					i++
					break
				}
				codeLines = append(codeLines, current)
				i++
			}
			if !closed {
				return nil, fmt.Errorf("unclosed code fence")
			}
			nodes = append(nodes, norgNode{kind: norgCode, lang: lang, code: strings.Join(codeLines, "\n"), attrs: attrs})
			continue
		}

		if isTableStart(trimmed) {
			i++
			tableLines := make([]string, 0)
			closed := false
			for i < len(lines) {
				current := strings.TrimSpace(strings.TrimRight(lines[i], "\r"))
				if current == "@end" {
					closed = true
					i++
					break
				}
				tableLines = append(tableLines, strings.TrimRight(lines[i], "\r"))
				i++
			}
			if !closed {
				return nil, fmt.Errorf("unclosed @table block")
			}
			tableHTML, err := parseMarkdownWrapperTable(tableLines)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, norgNode{kind: norgTable, tableHTML: tableHTML, attrs: attrs})
			continue
		}

		if text, ok := parseQuoteLine(trimmed); ok {
			quoteLines := []string{text}
			i++
			for i < len(lines) {
				next := strings.TrimSpace(strings.TrimRight(lines[i], "\r"))
				parsed, qok := parseQuoteLine(next)
				if !qok {
					break
				}
				quoteLines = append(quoteLines, parsed)
				i++
			}
			nodes = append(nodes, norgNode{kind: norgQuote, text: strings.Join(quoteLines, " "), attrs: attrs})
			continue
		}

		if item, ok := parseDefinitionLine(trimmed); ok {
			defs := []norgDefinitionItem{item}
			i++
			for i < len(lines) {
				next := strings.TrimSpace(strings.TrimRight(lines[i], "\r"))
				parsed, dok := parseDefinitionLine(next)
				if !dok {
					break
				}
				defs = append(defs, parsed)
				i++
			}
			nodes = append(nodes, norgNode{kind: norgDefinitionList, definitionItems: defs, attrs: attrs})
			continue
		}

		if strings.HasPrefix(trimmed, "*** ") {
			state, text, ok := parseTaskLine(trimmed)
			if !ok {
				parts := strings.Fields(trimmed)
				if len(parts) >= 2 {
					return nil, fmt.Errorf("invalid task state %q", parts[1])
				}
				return nil, fmt.Errorf("invalid task state")
			}
			items := []norgTaskItem{{state: state, text: text}}
			i++
			for i < len(lines) {
				nextTrimmed := strings.TrimSpace(strings.TrimRight(lines[i], "\r"))
				if !strings.HasPrefix(nextTrimmed, "*** ") {
					break
				}
				nextState, nextText, nextOK := parseTaskLine(nextTrimmed)
				if !nextOK {
					nextParts := strings.Fields(nextTrimmed)
					if len(nextParts) >= 2 {
						return nil, fmt.Errorf("invalid task state %q", nextParts[1])
					}
					return nil, fmt.Errorf("invalid task state")
				}
				items = append(items, norgTaskItem{state: nextState, text: nextText})
				i++
			}
			nodes = append(nodes, norgNode{kind: norgTaskList, taskItems: items, attrs: attrs})
			continue
		}

		if src, ok := parseDotImageLine(trimmed); ok {
			alt := imageAltFromSrc(src)
			nodes = append(nodes, norgNode{kind: norgParagraph, text: fmt.Sprintf("![%s](%s)", alt, src), attrs: attrs})
			i++
			continue
		}

		if level, text, ok := parseHeadingLine(trimmed); ok {
			nodes = append(nodes, norgNode{kind: norgHeading, level: level, text: text, attrs: attrs})
			i++
			continue
		}

		if item, ok := parseUnorderedLine(trimmed); ok {
			items := []string{item}
			i++
			for i < len(lines) {
				nextItem, nextOK := parseUnorderedLine(strings.TrimSpace(strings.TrimRight(lines[i], "\r")))
				if !nextOK {
					break
				}
				items = append(items, nextItem)
				i++
			}
			nodes = append(nodes, norgNode{kind: norgUL, items: items, attrs: attrs})
			continue
		}

		if item, ok := parseOrderedLine(trimmed); ok {
			items := []string{item}
			i++
			for i < len(lines) {
				nextItem, nextOK := parseOrderedLine(strings.TrimSpace(strings.TrimRight(lines[i], "\r")))
				if !nextOK {
					break
				}
				items = append(items, nextItem)
				i++
			}
			nodes = append(nodes, norgNode{kind: norgOL, items: items, attrs: attrs})
			continue
		}

		paragraphLines := []string{trimmed}
		i++
		for i < len(lines) {
			next := strings.TrimSpace(strings.TrimRight(lines[i], "\r"))
			if _, ok := parseAtCodeStart(next); next == "" || ok || isTableStart(next) || isFenceStart(next) || isTaskLine(next) || isDotImageLine(next) || isHeadingLine(next) || isUnorderedLine(next) || isOrderedLine(next) {
				break
			}
			if _, qok := parseQuoteLine(next); qok {
				break
			}
			if _, dok := parseDefinitionLine(next); dok {
				break
			}
			if _, _, mok := parseCarryoverMeta(next); mok {
				break
			}
			paragraphLines = append(paragraphLines, next)
			i++
		}
		nodes = append(nodes, norgNode{kind: norgParagraph, text: strings.Join(paragraphLines, " "), attrs: attrs})
	}
	return nodes, nil
}

func renderNorgHTML(nodes []norgNode) (string, error) {
	var b strings.Builder
	for _, n := range nodes {
		attrs := renderAttrs(n.attrs)
		switch n.kind {
		case norgHeading:
			lvl := n.level
			if lvl < 1 {
				lvl = 1
			}
			if lvl > 6 {
				lvl = 6
			}
			text, err := renderInline(n.text)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&b, "<h%d%s>%s</h%d>\n", lvl, attrs, text, lvl)
		case norgParagraph:
			text, err := renderInline(n.text)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&b, "<p%s>%s</p>\n", attrs, text)
		case norgUL:
			fmt.Fprintf(&b, "<ul%s>\n", attrs)
			for _, item := range n.items {
				text, err := renderInline(item)
				if err != nil {
					return "", err
				}
				fmt.Fprintf(&b, "<li>%s</li>\n", text)
			}
			b.WriteString("</ul>\n")
		case norgOL:
			fmt.Fprintf(&b, "<ol%s>\n", attrs)
			for _, item := range n.items {
				text, err := renderInline(item)
				if err != nil {
					return "", err
				}
				fmt.Fprintf(&b, "<li>%s</li>\n", text)
			}
			b.WriteString("</ol>\n")
		case norgTaskList:
			fmt.Fprintf(&b, "<ul class=\"task-list\"%s>\n", attrs)
			for _, item := range n.taskItems {
				text, err := renderInline(item.text)
				if err != nil {
					return "", err
				}
				fmt.Fprintf(&b, "<li data-task-state=\"%s\">%s</li>\n", renderTaskState(item.state), text)
			}
			b.WriteString("</ul>\n")
		case norgCode:
			if attrs != "" {
				fmt.Fprintf(&b, "<div%s>", attrs)
			}
			b.WriteString(renderHighlightedCode(n.lang, n.code))
			if attrs != "" {
				b.WriteString("</div>")
			}
			b.WriteString("\n")
		case norgTable:
			if attrs != "" {
				fmt.Fprintf(&b, "<div%s>", attrs)
			}
			b.WriteString(n.tableHTML)
			if attrs != "" {
				b.WriteString("</div>")
			}
			b.WriteString("\n")
		case norgQuote:
			text, err := renderInline(n.text)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&b, "<blockquote%s><p>%s</p></blockquote>\n", attrs, text)
		case norgDefinitionList:
			fmt.Fprintf(&b, "<dl class=\"norg-definitions\"%s>\n", attrs)
			for _, d := range n.definitionItems {
				if d.term != "" {
					term, err := renderInline(d.term)
					if err != nil {
						return "", err
					}
					fmt.Fprintf(&b, "<dt>%s</dt>\n", term)
				}
				body, err := renderInline(d.body)
				if err != nil {
					return "", err
				}
				fmt.Fprintf(&b, "<dd>%s</dd>\n", body)
			}
			b.WriteString("</dl>\n")
		}
	}
	return strings.TrimSpace(b.String()), nil
}

func renderHighlightedCode(lang, code string) string {
	lexerName := strings.TrimSpace(lang)
	lexer := lexers.Get(lexerName)
	if lexer == nil {
		lexer = lexers.Analyse(code)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return plainCodeHTML(lang, code)
	}

	style := styles.Get("github")
	if style == nil {
		style = styles.Fallback
	}

	formatter := chromahtml.New(chromahtml.WithClasses(true))
	var out bytes.Buffer
	if err := formatter.Format(&out, style, iterator); err != nil {
		return plainCodeHTML(lang, code)
	}

	rendered := strings.TrimSpace(out.String())
	rendered = chromaPreStylePattern.ReplaceAllString(rendered, `<pre class="chroma">`)
	return rendered
}

func plainCodeHTML(lang, code string) string {
	langClass := ""
	if lang != "" {
		langClass = fmt.Sprintf(" class=\"language-%s\"", html.EscapeString(lang))
	}
	return fmt.Sprintf("<pre><code%s>%s</code></pre>", langClass, html.EscapeString(code))
}

func renderTaskState(state string) string {
	switch state {
	case "TODO":
		return "todo"
	case "DOING":
		return "doing"
	case "DONE":
		return "done"
	case "CANCELLED":
		return "cancelled"
	default:
		return "todo"
	}
}

var (
	chromaPreStylePattern   = regexp.MustCompile(`<pre style="[^"]*">`)
	norgImagePattern        = regexp.MustCompile(`!\{([^}]+)\}\[([^\]]*)\]|\{([^}]+)\}\[([^\]]*)\]\(image\)`)
	inlinePattern           = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)|\[([^\]]+)\]\(([^)]+)\)`)
	tableSeparatorCellRegex = regexp.MustCompile(`^:?-+:?$`)
	carryoverPattern        = regexp.MustCompile(`^#([a-zA-Z0-9_-]+)\s+(.+)$`)
)

func renderInline(text string) (string, error) {
	text = normalizeNorgImageInline(text)
	idxs := inlinePattern.FindAllStringSubmatchIndex(text, -1)
	if len(idxs) == 0 {
		return renderTextWithModifiers(text), nil
	}

	var b strings.Builder
	cursor := 0
	for _, idx := range idxs {
		start := idx[0]
		end := idx[1]
		b.WriteString(renderTextWithModifiers(text[cursor:start]))

		if idx[2] != -1 && idx[4] != -1 {
			alt := html.EscapeString(text[idx[2]:idx[3]])
			rawSrc := text[idx[4]:idx[5]]
			src, err := validateCDNImageURL(rawSrc)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&b, `<img class="post-image" src="%s" alt="%s" loading="lazy" decoding="async"/>`, html.EscapeString(src), alt)
		} else {
			label := renderTextWithModifiers(text[idx[6]:idx[7]])
			href := html.EscapeString(text[idx[8]:idx[9]])
			fmt.Fprintf(&b, `<a href="%s">%s</a>`, href, label)
		}
		cursor = end
	}
	b.WriteString(renderTextWithModifiers(text[cursor:]))
	return b.String(), nil
}

func renderTextWithModifiers(raw string) string {
	var b strings.Builder
	for i := 0; i < len(raw); {
		delim := raw[i]
		if !isModifierDelimiter(delim) || isEscaped(raw, i) {
			if delim == '\\' && i+1 < len(raw) && isModifierDelimiter(raw[i+1]) {
				b.WriteString(html.EscapeString(string(raw[i+1])))
				i += 2
				continue
			}
			r, size := utf8.DecodeRuneInString(raw[i:])
			b.WriteString(html.EscapeString(string(r)))
			i += size
			continue
		}

		j := findClosingModifier(raw, i+1, delim)
		if j == -1 {
			b.WriteString(html.EscapeString(string(delim)))
			i++
			continue
		}

		inner := raw[i+1 : j]
		switch delim {
		case '*':
			fmt.Fprintf(&b, "<strong>%s</strong>", renderTextWithModifiers(inner))
		case '/':
			fmt.Fprintf(&b, "<em>%s</em>", renderTextWithModifiers(inner))
		case '_':
			fmt.Fprintf(&b, "<u>%s</u>", renderTextWithModifiers(inner))
		case '!':
			fmt.Fprintf(&b, "<span class=\"spoiler\">%s</span>", renderTextWithModifiers(inner))
		case '$':
			fmt.Fprintf(&b, "<span class=\"math-latex\">%s</span>", html.EscapeString(inner))
		}
		i = j + 1
	}
	return b.String()
}

func isModifierDelimiter(ch byte) bool {
	switch ch {
	case '*', '/', '_', '!', '$':
		return true
	default:
		return false
	}
}

func isEscaped(s string, index int) bool {
	if index <= 0 {
		return false
	}
	count := 0
	for i := index - 1; i >= 0 && s[i] == '\\'; i-- {
		count++
	}
	return count%2 == 1
}

func findClosingModifier(s string, start int, delim byte) int {
	for i := start; i < len(s); i++ {
		if s[i] == '\n' {
			return -1
		}
		if s[i] == delim && !isEscaped(s, i) {
			if i == start {
				return -1
			}
			return i
		}
	}
	return -1
}

func parseCarryoverMeta(line string) (string, string, bool) {
	m := carryoverPattern.FindStringSubmatch(line)
	if len(m) != 3 {
		return "", "", false
	}
	return strings.ToLower(strings.TrimSpace(m[1])), strings.TrimSpace(m[2]), true
}

func takeAttrs(attrs map[string]string) map[string]string {
	if len(attrs) == 0 {
		return nil
	}
	out := make(map[string]string, len(attrs))
	for k, v := range attrs {
		out[k] = v
		delete(attrs, k)
	}
	return out
}

func renderAttrs(attrs map[string]string) string {
	if len(attrs) == 0 {
		return ""
	}
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, " data-%s=\"%s\"", html.EscapeString(k), html.EscapeString(attrs[k]))
	}
	return b.String()
}

func parseQuoteLine(line string) (string, bool) {
	if !strings.HasPrefix(line, ">") {
		return "", false
	}
	text := strings.TrimSpace(line)
	for strings.HasPrefix(text, ">") {
		text = strings.TrimSpace(strings.TrimPrefix(text, ">"))
	}
	return text, true
}

func parseDefinitionLine(line string) (norgDefinitionItem, bool) {
	if !strings.HasPrefix(line, "$") {
		return norgDefinitionItem{}, false
	}
	raw := strings.TrimSpace(strings.TrimPrefix(line, "$"))
	if raw == "" {
		return norgDefinitionItem{}, false
	}
	if term, body, ok := strings.Cut(raw, ":"); ok {
		if strings.TrimSpace(body) != "" {
			return norgDefinitionItem{term: strings.TrimSpace(term), body: strings.TrimSpace(body)}, true
		}
	}
	return norgDefinitionItem{body: raw}, true
}

func validateCDNImageURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("invalid image url %q", raw)
	}
	if u.Scheme != "https" || u.Host == "" {
		return "", fmt.Errorf("image url must be https CDN url: %q", raw)
	}

	host := strings.ToLower(u.Hostname())
	if !strings.Contains(host, "cdn") {
		return "", fmt.Errorf("image host must be CDN: %q", raw)
	}

	cleanPath := path.Clean("/" + strings.TrimPrefix(u.Path, "/"))
	if strings.Contains(cleanPath, "..") {
		return "", fmt.Errorf("image path traversal blocked: %q", raw)
	}

	u.Path = cleanPath
	u.RawPath = ""
	return u.String(), nil
}

func isFenceStart(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "```")
}

func parseTaskLine(line string) (string, string, bool) {
	if !strings.HasPrefix(line, "*** ") {
		return "", "", false
	}
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return "", "", false
	}
	state := parts[1]
	if _, ok := allowedTaskStates[state]; !ok {
		return "", "", false
	}
	return state, strings.Join(parts[2:], " "), true
}

func isTaskLine(line string) bool {
	_, _, ok := parseTaskLine(line)
	return ok
}

func parseHeadingLine(line string) (int, string, bool) {
	if !isHeadingLine(line) {
		return 0, "", false
	}
	level := 0
	for level < len(line) && line[level] == '*' {
		level++
	}
	return level, strings.TrimSpace(line[level:]), true
}

func isHeadingLine(line string) bool {
	if line == "" || line[0] != '*' {
		return false
	}
	i := 0
	for i < len(line) && line[i] == '*' {
		i++
	}
	return i < len(line) && line[i] == ' '
}

func parseUnorderedLine(line string) (string, bool) {
	if !strings.HasPrefix(line, "- ") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(line, "- ")), true
}

func isUnorderedLine(line string) bool {
	_, ok := parseUnorderedLine(line)
	return ok
}

func isTableStart(line string) bool {
	return strings.TrimSpace(line) == "@table"
}

func parseMarkdownWrapperTable(lines []string) (string, error) {
	rows := make([][]string, 0, len(lines))
	for _, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		cells, ok := parseTableRow(trimmed)
		if !ok {
			return "", fmt.Errorf("invalid @table row %q", raw)
		}
		rows = append(rows, cells)
	}
	if len(rows) < 2 {
		return "", fmt.Errorf("invalid @table structure")
	}

	headers := rows[0]
	sep := rows[1]
	if len(headers) == 0 || len(headers) != len(sep) {
		return "", fmt.Errorf("invalid @table structure")
	}
	for _, cell := range sep {
		if !tableSeparatorCellRegex.MatchString(strings.TrimSpace(cell)) {
			return "", fmt.Errorf("invalid @table separator")
		}
	}

	var b strings.Builder
	b.WriteString("<table>\n<thead><tr>")
	for _, header := range headers {
		text, err := renderInline(header)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "<th>%s</th>", text)
	}
	b.WriteString("</tr></thead>\n<tbody>\n")
	for _, row := range rows[2:] {
		if len(row) != len(headers) {
			return "", fmt.Errorf("invalid @table row width")
		}
		b.WriteString("<tr>")
		for _, cell := range row {
			text, err := renderInline(cell)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&b, "<td>%s</td>", text)
		}
		b.WriteString("</tr>\n")
	}
	b.WriteString("</tbody>\n</table>")
	return b.String(), nil
}

func parseTableRow(line string) ([]string, bool) {
	if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
		return nil, false
	}
	parts := strings.Split(line, "|")
	if len(parts) < 3 {
		return nil, false
	}
	cells := make([]string, 0, len(parts)-2)
	for _, part := range parts[1 : len(parts)-1] {
		cells = append(cells, strings.TrimSpace(part))
	}
	return cells, true
}

func parseAtCodeStart(line string) (string, bool) {
	if !strings.HasPrefix(line, "@code") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(line, "@code")), true
}

func parseDotImageLine(line string) (string, bool) {
	if !strings.HasPrefix(line, ".image ") {
		return "", false
	}
	src := strings.TrimSpace(strings.TrimPrefix(line, ".image"))
	if src == "" {
		return "", false
	}
	return src, true
}

func isDotImageLine(line string) bool {
	_, ok := parseDotImageLine(strings.TrimSpace(line))
	return ok
}

func imageAltFromSrc(src string) string {
	clean := strings.TrimSpace(src)
	if clean == "" {
		return "image"
	}
	parts := strings.Split(clean, "/")
	name := parts[len(parts)-1]
	if dot := strings.LastIndex(name, "."); dot > 0 {
		name = name[:dot]
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "image"
	}
	return name
}

func normalizeNorgImageInline(text string) string {
	idxs := norgImagePattern.FindAllStringSubmatchIndex(text, -1)
	if len(idxs) == 0 {
		return text
	}

	var b strings.Builder
	cursor := 0
	for _, idx := range idxs {
		start := idx[0]
		end := idx[1]
		b.WriteString(text[cursor:start])

		src := ""
		alt := ""
		if idx[2] != -1 {
			src = strings.TrimSpace(text[idx[2]:idx[3]])
			alt = strings.TrimSpace(text[idx[4]:idx[5]])
		} else {
			src = strings.TrimSpace(text[idx[6]:idx[7]])
			alt = strings.TrimSpace(text[idx[8]:idx[9]])
		}
		if alt == "" {
			alt = imageAltFromSrc(src)
		}
		fmt.Fprintf(&b, "![%s](%s)", alt, src)
		cursor = end
	}
	b.WriteString(text[cursor:])
	return b.String()
}

func parseOrderedLine(line string) (string, bool) {
	idx := strings.Index(line, ".")
	if idx <= 0 || idx+1 >= len(line) || line[idx+1] != ' ' {
		return "", false
	}
	if _, err := strconv.Atoi(line[:idx]); err != nil {
		return "", false
	}
	return strings.TrimSpace(line[idx+1:]), true
}

func isOrderedLine(line string) bool {
	_, ok := parseOrderedLine(line)
	return ok
}

var allowedTaskStates = map[string]struct{}{
	"TODO":      {},
	"DOING":     {},
	"DONE":      {},
	"CANCELLED": {},
}
