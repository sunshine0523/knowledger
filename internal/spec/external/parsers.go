package external

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strconv"

	"github.com/kindbrave/claude-knowledger/internal/core"
)

// Parse dispatches to the right parser based on the parser name.
func Parse(parser, specID string, data []byte) ([]core.Finding, error) {
	switch parser {
	case "golangci-lint":
		return parseGolangciLint(specID, data)
	case "checkstyle":
		return parseCheckstyle(specID, data)
	case "eslint":
		return parseESLint(specID, data)
	default:
		return parseGenericJSON(specID, data)
	}
}

// --- golangci-lint JSON output ---

type golangciOutput struct {
	Issues []golangciIssue `json:"Issues"`
}

type golangciIssue struct {
	FromLinter  string           `json:"FromLinter"`
	Text        string           `json:"Text"`
	SourceLines []string         `json:"SourceLines"`
	Pos         golangciPosition `json:"Pos"`
}

type golangciPosition struct {
	Filename string `json:"Filename"`
	Line     int    `json:"Line"`
}

func parseGolangciLint(specID string, data []byte) ([]core.Finding, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var out golangciOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("golangci-lint: %w", err)
	}
	findings := make([]core.Finding, 0, len(out.Issues))
	for _, issue := range out.Issues {
		findings = append(findings, core.Finding{
			SpecID:   specID,
			RuleID:   issue.FromLinter,
			Path:     issue.Pos.Filename,
			Line:     issue.Pos.Line,
			Severity: "should-fix",
			Message:  issue.Text,
		})
	}
	return findings, nil
}

// --- checkstyle XML output ---

type checkstyleResult struct {
	XMLName xml.Name          `xml:"checkstyle"`
	Files   []checkstyleFile  `xml:"file"`
}

type checkstyleFile struct {
	Name   string            `xml:"name,attr"`
	Errors []checkstyleError `xml:"error"`
}

type checkstyleError struct {
	Line     string `xml:"line,attr"`
	Column   string `xml:"column,attr"`
	Severity string `xml:"severity,attr"`
	Message  string `xml:"message,attr"`
	Source   string `xml:"source,attr"`
}

func parseCheckstyle(specID string, data []byte) ([]core.Finding, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var result checkstyleResult
	if err := xml.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("checkstyle: %w", err)
	}
	var findings []core.Finding
	for _, file := range result.Files {
		for _, e := range file.Errors {
			line, _ := strconv.Atoi(e.Line)
			sev := mapCheckstyleSeverity(e.Severity)
			findings = append(findings, core.Finding{
				SpecID:   specID,
				RuleID:   e.Source,
				Path:     file.Name,
				Line:     line,
				Severity: sev,
				Message:  e.Message,
			})
		}
	}
	return findings, nil
}

func mapCheckstyleSeverity(s string) string {
	switch s {
	case "error":
		return "must-fix"
	case "warning":
		return "should-fix"
	default:
		return "nit"
	}
}

// --- ESLint JSON output ---

type eslintFileResult struct {
	FilePath string          `json:"filePath"`
	Messages []eslintMessage `json:"messages"`
}

type eslintMessage struct {
	RuleID   string `json:"ruleId"`
	Severity int    `json:"severity"` // 1=warn, 2=error
	Message  string `json:"message"`
	Line     int    `json:"line"`
}

func parseESLint(specID string, data []byte) ([]core.Finding, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var results []eslintFileResult
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("eslint: %w", err)
	}
	var findings []core.Finding
	for _, r := range results {
		for _, m := range r.Messages {
			sev := "should-fix"
			if m.Severity == 2 {
				sev = "must-fix"
			}
			findings = append(findings, core.Finding{
				SpecID:   specID,
				RuleID:   m.RuleID,
				Path:     r.FilePath,
				Line:     m.Line,
				Severity: sev,
				Message:  m.Message,
			})
		}
	}
	return findings, nil
}

// --- generic-json: expects []Finding directly ---

func parseGenericJSON(specID string, data []byte) ([]core.Finding, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var findings []core.Finding
	if err := json.Unmarshal(data, &findings); err != nil {
		return nil, fmt.Errorf("generic-json: %w", err)
	}
	for i := range findings {
		if findings[i].SpecID == "" {
			findings[i].SpecID = specID
		}
	}
	return findings, nil
}
