package plugin

import (
	"res-downloader/internal/logging"
	shared "res-downloader/internal/model"
)

type testResourceSink struct{}

func (testResourceSink) FilterSelectedCandidates(candidates []shared.ResourceCandidate) []shared.ResourceCandidate {
	return candidates
}
func (testResourceSink) PublishCandidate(shared.ResourceCandidate)    {}
func (testResourceSink) PublishCandidates([]shared.ResourceCandidate) {}
func (testResourceSink) CandidateByGroup(string, string) (shared.ResourceCandidate, bool) {
	return shared.ResourceCandidate{}, false
}
func (testResourceSink) RegisterTypes([]string) {}

func NewLogger(logFile bool, logPath string) *Logger { return logging.New(logFile, logPath) }
