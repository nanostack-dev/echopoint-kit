package apispec

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

const defaultIndentUnit = 2

const (
	minIndentUnit = 2
	maxIndentUnit = 9
)

// editSource is the text of the document an Edit call reads, with its line
// endings normalized to \n for the splice and restored on the way out.
type editSource struct {
	text []byte
	crlf bool
}

func newEditSource(data []byte) editSource {
	if bytes.Contains(data, []byte("\r\n")) {
		return editSource{text: bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")), crlf: true}
	}
	return editSource{text: data}
}

func (s editSource) restore(text []byte) []byte {
	if s.crlf {
		return bytes.ReplaceAll(text, []byte("\n"), []byte("\r\n"))
	}
	return text
}

// render writes the edited tree. A JSON document is written again as JSON. A
// YAML document is spliced: only the lines of the nodes that differ between
// the original tree and the edited tree change, and the rest of the original
// bytes stay as they are. When the splice cannot express the change, or its
// result does not read back as the edited tree, the whole document is encoded
// again.
func (s editSource) render(original, work *yaml.Node) ([]byte, error) {
	oldRoot, newRoot := original.Content[0], work.Content[0]
	if isFlow(oldRoot) {
		out, err := writeJSON(newRoot, jsonIndent(s.text), bytes.HasSuffix(s.text, []byte("\n")))
		return s.restore(out), err
	}
	unit := detectIndent(original)
	splice := newSplicer(s.text, unit)
	if splice.diffMapping(oldRoot, newRoot) && splice.err == nil {
		out := splice.apply()
		if readsBackAs(out, newRoot) {
			return s.restore(out), nil
		}
	}
	out, err := encodeNode(work, unit)
	return s.restore(out), err
}

func readsBackAs(text []byte, want *yaml.Node) bool {
	root, err := decodeRoot(text)
	return err == nil && sameData(root, want)
}

func encodeNode(node *yaml.Node, unit int) ([]byte, error) {
	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(unit)
	if err := encoder.Encode(node); err != nil {
		return nil, fmt.Errorf("encode the edited document: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("encode the edited document: %w", err)
	}
	return out.Bytes(), nil
}

// detectIndent is the indentation step between the first mapping and a
// mapping child written on the lines below it.
func detectIndent(node *yaml.Node) int {
	if step := indentStep(node); step >= minIndentUnit && step <= maxIndentUnit {
		return step
	}
	return defaultIndentUnit
}

func indentStep(node *yaml.Node) int {
	if node.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(node.Content); i += nodesPerEntry {
			key, value := node.Content[i], node.Content[i+1]
			if isBlockCollection(value, yaml.MappingNode) && value.Line > key.Line {
				return value.Column - key.Column
			}
		}
	}
	for _, child := range node.Content {
		if node.Kind != yaml.AliasNode {
			if step := indentStep(child); step != 0 {
				return step
			}
		}
	}
	return 0
}

type spliceEdit struct {
	start, end int
	text       []string
}

// splicer collects line edits of the original text. A line edit replaces the
// lines [start, end) with text, so an insertion has start == end.
//
// The algorithm walks the original tree and the edited tree together:
//   - Block mappings match their entries by key. A removed key deletes its
//     lines, an added key is inserted after the entry before it, and an entry
//     whose value changed recurses when both values are block collections of
//     one kind, or else is written again in place. A single renamed key only
//     rewrites the key. Surviving keys that changed order replace the parent.
//   - Block sequences recurse item by item when the length is equal, handle one
//     inserted or removed item, and otherwise replace the parent.
//   - Flow collections are atomic: a change inside one replaces the entry that
//     holds it.
//
// The lines of an entry run from its key (or dash) to the last line indented
// deeper than it, plus a block sequence written at the key's own indentation.
// Comment lines directly above an entry belong to it. Trailing blank lines do
// not.
type splicer struct {
	lines           []string
	trailingNewline bool
	unit            int
	edits           []spliceEdit
	err             error
}

func newSplicer(text []byte, unit int) *splicer {
	lines := strings.Split(string(text), "\n")
	trailing := len(lines) > 0 && lines[len(lines)-1] == ""
	if trailing {
		lines = lines[:len(lines)-1]
	}
	return &splicer{lines: lines, trailingNewline: trailing, unit: unit}
}

func (s *splicer) apply() []byte {
	slices.SortStableFunc(s.edits, func(a, b spliceEdit) int {
		if a.start != b.start {
			return a.start - b.start
		}
		return a.end - b.end
	})
	out := make([]string, 0, len(s.lines))
	cursor := 0
	for _, edit := range s.edits {
		if edit.start > cursor {
			out = append(out, s.lines[cursor:edit.start]...)
			cursor = edit.start
		}
		out = append(out, edit.text...)
		cursor = max(cursor, edit.end)
	}
	out = append(out, s.lines[cursor:]...)
	text := strings.Join(out, "\n")
	if s.trailingNewline {
		text += "\n"
	}
	return []byte(text)
}

func (s *splicer) indentOf(line int) int {
	text := s.lines[line]
	return len(text) - len(strings.TrimLeft(text, " "))
}

func (s *splicer) isBlank(line int) bool {
	return strings.TrimSpace(s.lines[line]) == ""
}

func (s *splicer) isComment(line int) bool {
	return strings.HasPrefix(strings.TrimSpace(s.lines[line]), "#")
}

func isDashAt(line string, col int) bool {
	return len(line) > col && line[col] == '-' && (len(line) == col+1 || line[col+1] == ' ')
}

// entry is a mapping entry or a sequence item with the lines it occupies.
// key is nil for a sequence item. end is the last line, inclusive.
type entry struct {
	key, value *yaml.Node
	col        int
	start, end int
	head       int
	inline     bool
}

func (s *splicer) newEntry(key, value *yaml.Node, start, col int, indentless bool) entry {
	found := entry{key: key, value: value, col: col, start: start, head: start}
	found.end = s.entryEnd(start, col, indentless)
	found.inline = strings.TrimSpace(s.lines[start][:col]) != ""
	if !found.inline {
		found.head = s.headStart(start, col)
	}
	return found
}

func (s *splicer) entryEnd(start, col int, indentless bool) int {
	end := start
	for i := start + 1; i < len(s.lines); i++ {
		if s.isBlank(i) {
			continue
		}
		indent := s.indentOf(i)
		if indent > col || (indentless && indent == col && isDashAt(s.lines[i], col)) {
			end = i
			continue
		}
		break
	}
	return end
}

func (s *splicer) headStart(start, col int) int {
	head := start
	for head > 0 && s.isComment(head-1) && s.indentOf(head-1) <= col {
		head--
	}
	return head
}

func (s *splicer) mappingEntries(mapping *yaml.Node) ([]entry, bool) {
	entries := make([]entry, 0, len(mapping.Content)/nodesPerEntry)
	for i := 0; i+1 < len(mapping.Content); i += nodesPerEntry {
		key, value := mapping.Content[i], mapping.Content[i+1]
		start, col := key.Line-1, key.Column-1
		if start < 0 || start >= len(s.lines) || col < 0 || col > len(s.lines[start]) {
			return nil, false
		}
		indentless := value.Kind == yaml.SequenceNode && !isFlow(value) && value.Column-1 == col && value.Line-1 > start
		entries = append(entries, s.newEntry(key, value, start, col, indentless))
	}
	return entries, true
}

func (s *splicer) sequenceEntries(sequence *yaml.Node) ([]entry, bool) {
	first, col := sequence.Line-1, sequence.Column-1
	if first < 0 || first >= len(s.lines) || col < 0 || !isDashAt(s.lines[first], col) {
		return nil, false
	}
	starts := []int{first}
	for i := first + 1; i < len(s.lines) && len(starts) < len(sequence.Content); i++ {
		if s.indentOf(i) == col && isDashAt(s.lines[i], col) {
			starts = append(starts, i)
		}
	}
	if len(starts) != len(sequence.Content) {
		return nil, false
	}
	entries := make([]entry, 0, len(starts))
	for index, item := range sequence.Content {
		entries = append(entries, s.newEntry(nil, item, starts[index], col, false))
	}
	return entries, true
}

func recursable(oldValue, newValue *yaml.Node) bool {
	return (isBlockCollection(oldValue, yaml.MappingNode) && isBlockCollection(newValue, yaml.MappingNode)) ||
		(isBlockCollection(oldValue, yaml.SequenceNode) && isBlockCollection(newValue, yaml.SequenceNode))
}

func (s *splicer) diffCollection(oldValue, newValue *yaml.Node) bool {
	if oldValue.Kind == yaml.MappingNode {
		return s.diffMapping(oldValue, newValue)
	}
	return s.diffSequence(oldValue, newValue)
}

// diffEntry brings one entry to its new key and value. It always succeeds: when
// the entry cannot be edited inside, its lines are written again.
func (s *splicer) diffEntry(old entry, newKey, newValue *yaml.Node) {
	renamed := old.key != nil && old.key.Value != newKey.Value
	if nodesEqual(old.value, newValue) {
		if !renamed || s.renameKey(old, newKey) {
			return
		}
	} else if recursable(old.value, newValue) {
		mark := len(s.edits)
		if s.diffCollection(old.value, newValue) && (!renamed || s.renameKey(old, newKey)) {
			return
		}
		s.edits = s.edits[:mark]
	}
	s.replaceEntry(old, newKey, newValue)
}

func (s *splicer) replaceEntry(old entry, newKey, newValue *yaml.Node) {
	lines := s.entryLines(newKey, newValue, old.col)
	if len(lines) == 0 {
		return
	}
	if prefix := s.lines[old.start][:old.col]; strings.TrimSpace(prefix) != "" {
		lines[0] = prefix + lines[0][old.col:]
	}
	s.edits = append(s.edits, spliceEdit{start: old.start, end: old.end + 1, text: lines})
}

// renameKey rewrites the key in place, on its own line.
func (s *splicer) renameKey(old entry, newKey *yaml.Node) bool {
	line := s.lines[old.start]
	length, ok := sourceScalarLength(line[old.col:], old.key)
	if !ok {
		return false
	}
	rest := line[old.col+length:]
	if !strings.HasPrefix(strings.TrimLeft(rest, " "), ":") {
		return false
	}
	text, ok := s.keyText(newKey)
	if !ok {
		return false
	}
	s.edits = append(s.edits, spliceEdit{
		start: old.start, end: old.start + 1, text: []string{line[:old.col] + text + rest},
	})
	return true
}

func (s *splicer) keyText(key *yaml.Node) (string, bool) {
	bare := *key
	bare.HeadComment, bare.LineComment, bare.FootComment = "", "", ""
	lines := s.encodeLines(&yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{&bare, strNode("x")}})
	if len(lines) != 1 || !strings.HasSuffix(lines[0], ": x") {
		return "", false
	}
	return strings.TrimSuffix(lines[0], ": x"), true
}

// sourceScalarLength is how many bytes a scalar takes in the source text.
func sourceScalarLength(text string, node *yaml.Node) (int, bool) {
	switch {
	case node.Style&yaml.DoubleQuotedStyle != 0:
		return quotedLength(text, '"', false)
	case node.Style&yaml.SingleQuotedStyle != 0:
		return quotedLength(text, '\'', true)
	case node.Style&(yaml.LiteralStyle|yaml.FoldedStyle|yaml.FlowStyle) != 0:
		return 0, false
	}
	return len(node.Value), node.Value != "" && strings.HasPrefix(text, node.Value)
}

func quotedLength(text string, quote byte, doubled bool) (int, bool) {
	if text == "" || text[0] != quote {
		return 0, false
	}
	for i := 1; i < len(text); i++ {
		switch {
		case !doubled && text[i] == '\\':
			i++
		case text[i] == quote && doubled && i+1 < len(text) && text[i+1] == quote:
			i++
		case text[i] == quote:
			return i + 1, true
		}
	}
	return 0, false
}

func (s *splicer) encodeLines(node *yaml.Node) []string {
	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(s.unit)
	err := encoder.Encode(node)
	if closeErr := encoder.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		s.err = err
		return nil
	}
	return strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
}

// entryLines encodes a mapping entry, or a sequence item when key is nil,
// indented to a column. The comments above and below the entry stay where
// they are in the source, so they are not written again.
func (s *splicer) entryLines(key, value *yaml.Node, col int) []string {
	bareValue := *value
	var node *yaml.Node
	if key == nil {
		bareValue.HeadComment, bareValue.FootComment = "", ""
		node = &yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{&bareValue}}
	} else {
		bareKey := *key
		bareKey.HeadComment, bareKey.FootComment = "", ""
		node = &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{&bareKey, &bareValue}}
	}
	lines := s.encodeLines(node)
	padding := strings.Repeat(" ", col)
	for i, line := range lines {
		if line != "" {
			lines[i] = padding + line
		}
	}
	return lines
}

func (s *splicer) diffMapping(oldMapping, newMapping *yaml.Node) bool {
	entries, ok := s.mappingEntries(oldMapping)
	if !ok {
		return false
	}
	oldKeys, newKeys := keyTexts(oldMapping), keyTexts(newMapping)
	if len(oldKeys) == len(newKeys) && differAtMostOnce(oldKeys, newKeys) {
		for i, old := range entries {
			s.diffEntry(old, newMapping.Content[i*nodesPerEntry], newMapping.Content[i*nodesPerEntry+1])
		}
		return true
	}
	if hasDuplicates(oldKeys) || hasDuplicates(newKeys) {
		return false
	}
	return s.diffMappingByKey(entries, oldKeys, newMapping, newKeys)
}

func keyTexts(mapping *yaml.Node) []string {
	keys := make([]string, 0, len(mapping.Content)/nodesPerEntry)
	for i := 0; i < len(mapping.Content); i += nodesPerEntry {
		keys = append(keys, mapping.Content[i].Value)
	}
	return keys
}

func differAtMostOnce(a, b []string) bool {
	differences := 0
	for i := range a {
		if a[i] != b[i] {
			differences++
		}
	}
	return differences <= 1
}

func hasDuplicates(keys []string) bool {
	seen := make(map[string]bool, len(keys))
	for _, key := range keys {
		if seen[key] {
			return true
		}
		seen[key] = true
	}
	return false
}

func (s *splicer) diffMappingByKey(entries []entry, oldKeys []string, newMapping *yaml.Node, newKeys []string) bool {
	oldIndex := make(map[string]int, len(oldKeys))
	for i, key := range oldKeys {
		oldIndex[key] = i
	}
	inNew := make(map[string]bool, len(newKeys))
	for _, key := range newKeys {
		inNew[key] = true
	}
	survivingOld := slices.DeleteFunc(slices.Clone(oldKeys), func(key string) bool { return !inNew[key] })
	survivingNew := slices.DeleteFunc(slices.Clone(newKeys), func(key string) bool {
		_, kept := oldIndex[key]
		return !kept
	})
	if len(survivingOld) == 0 || !slices.Equal(survivingOld, survivingNew) {
		return false
	}

	separated := s.separated(entries)
	for j, key := range newKeys {
		if i, kept := oldIndex[key]; kept {
			s.diffEntry(entries[i], newMapping.Content[j*nodesPerEntry], newMapping.Content[j*nodesPerEntry+1])
		}
	}
	for i, key := range oldKeys {
		if !inNew[key] && !s.deleteEntry(entries, i) {
			return false
		}
	}

	previous := -1
	var added []string
	for j, key := range newKeys {
		i, kept := oldIndex[key]
		if !kept {
			added = append(
				added,
				s.entryLines(newMapping.Content[j*nodesPerEntry], newMapping.Content[j*nodesPerEntry+1],
					entries[0].col)...)
			continue
		}
		if !s.insertLines(entries, previous, i, added, separated) {
			return false
		}
		previous, added = i, nil
	}
	return s.insertLines(entries, previous, -1, added, separated)
}

// separated says whether every pair of neighbours has a blank line between.
func (s *splicer) separated(entries []entry) bool {
	if len(entries) < minSeparatedEntries {
		return false
	}
	for i := 0; i+1 < len(entries); i++ {
		blank := false
		for line := entries[i].end + 1; line < entries[i+1].head; line++ {
			blank = blank || s.isBlank(line)
		}
		if !blank {
			return false
		}
	}
	return true
}

// deleteEntry removes an entry with the comment above it, and a blank line
// that kept it apart from a neighbour.
func (s *splicer) deleteEntry(entries []entry, i int) bool {
	if entries[i].inline {
		return false
	}
	start, end := entries[i].head, entries[i].end+1
	switch {
	case i+1 < len(entries):
		for end < entries[i+1].head && s.isBlank(end) {
			end++
		}
	case i > 0:
		for start-1 > entries[i-1].end && s.isBlank(start-1) {
			start--
		}
	}
	s.edits = append(s.edits, spliceEdit{start: start, end: end})
	return true
}

// insertLines writes new lines between the entries previous and next, either of
// which is -1 at the edge of the collection. When blank lines separate the
// entries, the new lines are kept apart too.
func (s *splicer) insertLines(entries []entry, previous, next int, lines []string, separated bool) bool {
	if len(lines) == 0 {
		return true
	}
	var position int
	switch {
	case next >= 0 && (previous < 0 || separated):
		if entries[next].inline {
			return false
		}
		position = entries[next].head
		if separated {
			lines = append(lines, "")
		}
	case next >= 0:
		position = entries[previous].end + 1
	default:
		position = entries[previous].end + 1
		if separated {
			lines = append([]string{""}, lines...)
		}
	}
	s.edits = append(s.edits, spliceEdit{start: position, end: position, text: lines})
	return true
}

func (s *splicer) diffSequence(oldSequence, newSequence *yaml.Node) bool {
	entries, ok := s.sequenceEntries(oldSequence)
	if !ok {
		return false
	}
	oldItems, newItems := oldSequence.Content, newSequence.Content
	switch len(newItems) - len(oldItems) {
	case 0:
		for i, old := range entries {
			s.diffEntry(old, nil, newItems[i])
		}
		return true
	case 1:
		at := firstDifference(oldItems, newItems)
		if !itemsEqual(oldItems[at:], newItems[at+1:]) {
			return false
		}
		next := -1
		if at < len(oldItems) {
			next = at
		}
		lines := s.entryLines(nil, newItems[at], entries[0].col)
		return s.insertLines(entries, at-1, next, lines, s.separated(entries))
	case -1:
		at := firstDifference(newItems, oldItems)
		if !itemsEqual(oldItems[at+1:], newItems[at:]) {
			return false
		}
		return s.deleteEntry(entries, at)
	}
	return false
}

func firstDifference(a, b []*yaml.Node) int {
	shared := min(len(a), len(b))
	for i := range shared {
		if !nodesEqual(a[i], b[i]) {
			return i
		}
	}
	return shared
}

func itemsEqual(a, b []*yaml.Node) bool {
	return slices.EqualFunc(a, b, nodesEqual)
}
