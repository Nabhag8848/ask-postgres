package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// appVersion is shown in the welcome panel; bump when tagging releases.
const appVersion = "0.1.0"

// elephantArt is the classic PostgreSQL elephant (text/footer from the original omitted).
var elephantArt = []string{
	`   ____  ______  ___    `,
	`  /    )/      \/   \   `,
	` (     / __    _\    )  `,
	`  \    (/ o)  ( o)   )  `,
	`   \_  (_  )   \ )  /   `,
	`     \  /\_/    \)_/    `,
	`      \/  //|  |\\      `,
	`          v |  | v      `,
	`            \__/        `,
}

// elephantArtMaxWidth is the widest display line in elephantArt (for layout; avoids lipgloss wrap).
var elephantArtMaxWidth int

func init() {
	for _, line := range elephantArt {
		if w := lipgloss.Width(line); w > elephantArtMaxWidth {
			elephantArtMaxWidth = w
		}
	}
}

func elephantLines() int { return len(elephantArt) }

func (m Model) shouldShowWelcome() bool {
	if m.streaming {
		return false
	}
	return strings.TrimSpace(m.outText) == ""
}

func welcomeUserName() string {
	if u := strings.TrimSpace(os.Getenv("USER")); u != "" {
		return u
	}
	if u := strings.TrimSpace(os.Getenv("USERNAME")); u != "" {
		return u
	}
	return "there"
}

func welcomeShortPath() string {
	wd, err := os.Getwd()
	if err != nil || strings.TrimSpace(wd) == "" {
		return "."
	}
	home, herr := os.UserHomeDir()
	if herr == nil && home != "" && strings.HasPrefix(wd, home) {
		return "~" + strings.TrimPrefix(wd, home)
	}
	return wd
}

func (m Model) welcomeContextLine(maxPlainWidth int) string {
	parts := []string{
		m.cfg.Model,
		"read-only Postgres",
		welcomeShortPath(),
	}
	line := strings.Join(parts, " · ")
	if maxPlainWidth > 0 {
		line = truncatePlainRunes(line, maxPlainWidth)
	}
	return line
}

// truncatePlainRunes shortens s to at most maxW runes (for plain text); adds … if trimmed.
func truncatePlainRunes(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= maxW {
		return s
	}
	if maxW <= 1 {
		return "…"
	}
	return string(r[:maxW-1]) + "…"
}

// styleElephant applies accent color to the whole ASCII block in one render (no Width wrap).
func styleElephant(th theme) string {
	return th.Accent.Render(strings.Join(elephantArt, "\n"))
}

// rightAlignBlock pads each line so the block sits flush right in a column of width colW.
func rightAlignBlock(block string, colW int) string {
	if colW < 1 {
		return block
	}
	lines := strings.Split(block, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, rightAlignLine(line, colW))
	}
	return strings.Join(out, "\n")
}

func rightAlignLine(line string, colW int) string {
	if colW < 1 {
		return line
	}
	w := lipgloss.Width(line)
	if w >= colW {
		return line
	}
	return strings.Repeat(" ", colW-w) + line
}

// centerBlock pads each logical line so the block is optically centered in totalW cells (stacked fallback).
func centerBlock(block string, totalW int) string {
	if totalW < 1 {
		return block
	}
	lines := strings.Split(block, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		w := lipgloss.Width(line)
		if w >= totalW {
			out = append(out, line)
			continue
		}
		pad := (totalW - w) / 2
		out = append(out, strings.Repeat(" ", pad)+line)
	}
	return strings.Join(out, "\n")
}

func welcomeRule(th theme, width int) string {
	if width < 2 {
		return ""
	}
	return th.Meta.Render(strings.Repeat("─", width))
}

func welcomeBullet(th theme, cmd, rest string) string {
	const gap = "  "
	return gap + th.Accent.Render(cmd) + th.Meta.Render(" — "+rest)
}

// welcomeMaxLineWidth returns the widest display line in a multiline block.
func welcomeMaxLineWidth(block string) int {
	m := 0
	for _, line := range strings.Split(block, "\n") {
		if w := lipgloss.Width(line); w > m {
			m = w
		}
	}
	return m
}

// welcomeSpacerColumn is `rows` identical lines of spaces of width `width`.
func welcomeSpacerColumn(rows, width int) string {
	if rows < 1 {
		return ""
	}
	if width < 1 {
		lines := make([]string, rows)
		return strings.Join(lines, "\n")
	}
	row := strings.Repeat(" ", width)
	lines := make([]string, rows)
	for i := range lines {
		lines[i] = row
	}
	return strings.Join(lines, "\n")
}

// welcomeTopBar is one row: greeting flush left, product name + version flush right (total width totalW).
func welcomeTopBar(th theme, totalW int, user string) string {
	greetPlain := fmt.Sprintf("Welcome back, %s!", user)
	right := th.Title.Render("ask-postgres") + th.Meta.Render("  v"+appVersion)
	rw := lipgloss.Width(right)
	if totalW < 1 {
		return right
	}
	needGap := 1
	maxRunes := len([]rune(greetPlain))
	for maxRunes >= 4 {
		left := th.User.Render(truncatePlainRunes(greetPlain, maxRunes))
		lw := lipgloss.Width(left)
		if lw+rw+needGap <= totalW {
			mid := totalW - lw - rw
			if mid < needGap {
				mid = needGap
			}
			return left + strings.Repeat(" ", mid) + right
		}
		maxRunes--
	}
	left := th.User.Render("Hi!")
	return left + strings.Repeat(" ", max(needGap, totalW-lipgloss.Width(left)-rw)) + right
}

// renderWelcomePanel draws a low, wide card: text on the left, elephant on the right, same height as the art.
// No outer Width() — lipgloss word-wrap breaks ASCII art.
func (m Model) renderWelcomePanel(maxW, maxH int) string {
	th := m.theme
	if maxW < 10 || maxH <= 0 {
		return lipgloss.NewStyle().Width(maxW).Render("")
	}

	// Border (2) + horizontal padding only Padding(0,2) => 4 cells horizontal.
	innerMax := max(12, maxW-6)

	content := m.welcomeContent(th, innerMax, maxH)
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(th.BorderColor).
		Padding(0, 2).
		Render(content)
}

// welcomeContent picks side-by-side layout when there is room; otherwise a short stacked block.
func (m Model) welcomeContent(th theme, innerMax, maxH int) string {
	const colGap = "  "
	gapW := lipgloss.Width(colGap)
	rightColW := elephantArtMaxWidth
	leftW := innerMax - gapW - rightColW
	minLeft := 22
	// Split adds a full-width top bar + rule above the elephant block.
	splitMinH := elephantLines() + 2

	if leftW >= minLeft && maxH >= splitMinH {
		return m.welcomeContentSplit(th, innerMax, leftW, rightColW, colGap, maxH)
	}
	return m.welcomeContentStacked(th, innerMax, maxH)
}

// welcomeContentSplit: full-width top row (greeting left, brand+version right), full-width rule, then left body | elephant (same height).
func (m Model) welcomeContentSplit(th theme, innerMax, leftW, rightColW int, colGap string, maxH int) string {
	h := elephantLines()
	innerTotal := innerMax

	top := welcomeTopBar(th, innerTotal, welcomeUserName())
	ruleFull := welcomeRule(th, innerTotal)

	section := th.Accent.Render("Start here")
	row1 := welcomeBullet(th, "/help", "commands & keys")
	row2 := welcomeBullet(th, "/", "palette")
	context := th.Meta.Render(m.welcomeContextLine(leftW))

	var leftLines []string
	if h == 9 {
		leftLines = make([]string, h)
		// Body only (greeting + brand in top bar): spacer rows, then actions mid, context bottom.
		leftLines[0], leftLines[1], leftLines[2] = "", "", ""
		leftLines[3] = section
		leftLines[4] = row1
		leftLines[5] = row2
		leftLines[6], leftLines[7] = "", ""
		leftLines[8] = context
	} else {
		leftLines = m.welcomeLeftColumnBalanced(th, leftW, h)
	}

	leftCol := strings.Join(leftLines, "\n")
	rightCol := rightAlignBlock(styleElephant(th), rightColW)
	// Eat slack so the elephant column hugs the inner right edge (extreme right).
	maxL := welcomeMaxLineWidth(leftCol)
	gapW := lipgloss.Width(colGap)
	spacerW := innerMax - maxL - gapW - rightColW
	if spacerW < 0 {
		spacerW = 0
	}
	spacerCol := welcomeSpacerColumn(h, spacerW)
	body := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, colGap, spacerCol, rightCol)
	inner := lipgloss.JoinVertical(lipgloss.Left, top, ruleFull, body)

	maxLines := max(1, maxH-2)
	return clampBlockHeight(inner, maxLines)
}

// welcomeLeftColumnBalanced builds a left column of h rows when elephant line count != 9 (no brand/greet; those are in the top bar).
func (m Model) welcomeLeftColumnBalanced(th theme, leftW, h int) []string {
	section := th.Accent.Render("Start here")
	row1 := welcomeBullet(th, "/help", "commands & keys")
	row2 := welcomeBullet(th, "/", "palette")
	context := th.Meta.Render(m.welcomeContextLine(leftW))

	lines := make([]string, h)
	lines[h-1] = context
	// Middle: section + bullets centered in remaining rows.
	body := []string{section, row1, row2}
	start := 0
	end := h - 1
	avail := end - start
	if avail < len(body) {
		for i := range body {
			if start+i < end {
				lines[start+i] = body[i]
			}
		}
		return lines
	}
	pad := (avail - len(body)) / 2
	for i := range body {
		lines[start+pad+i] = body[i]
	}
	return lines
}

// welcomeContentStacked: narrow terminals — same top bar (greet left / brand right), then stack.
func (m Model) welcomeContentStacked(th theme, innerMax, maxH int) string {
	user := welcomeUserName()
	top := welcomeTopBar(th, innerMax, user)
	rule := welcomeRule(th, innerMax)
	context := th.Meta.Render(m.welcomeContextLine(innerMax))
	section := th.Accent.Render("Start here")
	row1 := welcomeBullet(th, "/help", "commands & keybindings")
	row2 := welcomeBullet(th, "/", "command palette")

	var parts []string
	parts = append(parts, top, rule)
	if maxH >= 14 {
		parts = append(parts, "", centerBlock(styleElephant(th), innerMax))
	}
	parts = append(parts, "", context, "", section, row1, row2)

	inner := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return clampBlockHeight(inner, max(1, maxH-2))
}

func clampBlockHeight(block string, maxLines int) string {
	if maxLines <= 0 || block == "" {
		return block
	}
	lines := strings.Split(block, "\n")
	if len(lines) <= maxLines {
		return block
	}
	return strings.Join(lines[:maxLines], "\n")
}
