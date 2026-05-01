package analysis

import "time"

type Result struct {
	Summary     ScanSummary  `json:"summary"`
	Metrics     Metrics      `json:"metrics"`
	Suggestions []Suggestion `json:"suggestions"`
}

type ScanSummary struct {
	SessionRoot     string    `json:"session_root"`
	FilesScanned    int       `json:"files_scanned"`
	SessionsScanned int       `json:"sessions_scanned"`
	LinesScanned    int       `json:"lines_scanned"`
	EventsScanned   int       `json:"events_scanned"`
	MalformedLines  int       `json:"malformed_lines"`
	SkippedFiles    int       `json:"skipped_files"`
	FirstDate       time.Time `json:"first_date,omitempty"`
	LastDate        time.Time `json:"last_date,omitempty"`
}

type Metrics struct {
	SessionsByDay       map[string]int `json:"sessions_by_day"`
	EventTypes          map[string]int `json:"event_types"`
	Repositories        map[string]int `json:"repositories"`
	Models              map[string]int `json:"models"`
	Tools               map[string]int `json:"tools"`
	ShellCommands       int            `json:"shell_commands"`
	FailedShellCommands int            `json:"failed_shell_commands"`
	ApprovalEvents      int            `json:"approval_events"`
	ErrorEvents         int            `json:"error_events"`
	VerificationSignals int            `json:"verification_signals"`
	FinalResponses      int            `json:"final_responses"`
	UnknownEventTypes   int            `json:"unknown_event_types"`
	DataQuality         DataQuality    `json:"data_quality"`
}

type DataQuality struct {
	FilesScanned      int `json:"files_scanned"`
	LinesScanned      int `json:"lines_scanned"`
	MalformedLines    int `json:"malformed_lines"`
	UnknownEventTypes int `json:"unknown_event_types"`
	MissingTimestamps int `json:"missing_timestamps"`
}

type Suggestion struct {
	Title       string   `json:"title"`
	Severity    string   `json:"severity"`
	Confidence  float64  `json:"confidence"`
	Evidence    []string `json:"evidence"`
	Action      string   `json:"action"`
	AffectedSet string   `json:"affected_set"`
}
