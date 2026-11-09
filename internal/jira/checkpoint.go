package jira

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/ysksm/task-app/internal/service"
)

// checkpointPrefix starts the first line of a comment that is a checkpoint,
// both when checkpoints are shown as comments and when a Jira client posts
// one (docs/01-requirements.md §4.10.3).
const checkpointPrefix = "[Checkpoint]"

var (
	headingMarks = regexp.MustCompile(`^#+\s*`)
	seqMark      = regexp.MustCompile(`^#\d+\s*`)
	// "- **判断基準**: ...", "判断基準: ...", "* Rationale：..."
	labelLine = regexp.MustCompile(`^(?:[-*+]\s+)?(?:\*\*|__)?([^:：*_]+?)(?:\*\*|__)?\s*[:：]\s*(.*)$`)
)

var checkpointLabels = map[string]string{
	"判断": "decision", "decision": "decision",
	"判断基準": "rationale", "rationale": "rationale", "根拠": "rationale",
	"代替案": "alternatives", "alternatives": "alternatives",
	"確信度": "confidence", "confidence": "confidence",
	"進捗": "progress", "progress": "progress",
	"次": "next", "次のステップ": "next", "next": "next", "next steps": "next",
	"要約": "summary", "summary": "summary",
}

var confidenceValues = map[string]string{
	"高": "high", "high": "high", "中": "medium", "medium": "medium", "低": "low", "low": "low",
}

// parseCheckpoint reads a comment such as
//
//	[Checkpoint] ログインフォームを実装
//	- 判断: React Hook Form を使う
//	- 判断基準: 既存の依存で済む
//	- 確信度: 中
//
// isCheckpoint is false when the comment does not start with the prefix.
func parseCheckpoint(md string) (in service.CheckpointInput, isCheckpoint bool, err error) {
	lines := strings.Split(strings.TrimSpace(md), "\n")
	first := headingMarks.ReplaceAllString(strings.TrimSpace(lines[0]), "")
	first = strings.TrimPrefix(strings.TrimPrefix(first, "**"), "__")
	if !strings.HasPrefix(first, checkpointPrefix) {
		return in, false, nil
	}
	rest := strings.TrimSpace(strings.TrimPrefix(first, checkpointPrefix))
	rest = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(rest, "**"), "__"))
	in.Summary = strings.TrimSpace(seqMark.ReplaceAllString(rest, ""))
	var extra []string
	for _, line := range lines[1:] {
		line = strings.TrimSpace(strings.TrimSuffix(line, "  "))
		if line == "" {
			continue
		}
		mm := labelLine.FindStringSubmatch(line)
		field := ""
		if mm != nil {
			field = checkpointLabels[strings.ToLower(strings.TrimSpace(mm[1]))]
		}
		if field == "" {
			extra = append(extra, line)
			continue
		}
		v := strings.TrimSpace(mm[2])
		switch field {
		case "summary":
			in.Summary = v
		case "decision":
			in.Decision = v
		case "rationale":
			in.Rationale = v
		case "alternatives":
			for _, a := range regexp.MustCompile(`\s*(?:/|、|,)\s*`).Split(v, -1) {
				if a != "" {
					in.Alternatives = append(in.Alternatives, a)
				}
			}
		case "confidence":
			word := strings.ToLower(strings.Fields(v + " ")[0])
			for k, c := range confidenceValues {
				if strings.HasPrefix(word, k) {
					in.Confidence = c
				}
			}
			if in.Confidence == "" {
				return in, true, errors.New("確信度 (confidence) must be 高/中/低 or high/medium/low.")
			}
		case "progress":
			n, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(v, "%")))
			if err != nil {
				return in, true, errors.New("進捗 (progress) must be a number from 0 to 100.")
			}
			in.Progress = &n
		case "next":
			in.NextSteps = v
		}
	}
	if in.Summary == "" && len(extra) > 0 {
		in.Summary, extra = extra[0], extra[1:]
	}
	if in.Rationale == "" {
		return in, true, errors.New("A [Checkpoint] comment needs a line '判断基準: ...' (or 'Rationale: ...'). " +
			"Format: '[Checkpoint] <summary>' then lines such as '判断: ...', '判断基準: ...', '確信度: 高|中|低', '進捗: 40%', '次: ...'.")
	}
	if in.Summary == "" {
		return in, true, errors.New("A [Checkpoint] comment needs a summary after '[Checkpoint]'.")
	}
	return in, true, nil
}
