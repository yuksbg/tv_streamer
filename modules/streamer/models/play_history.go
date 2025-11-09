package models

import (
	"time"
)

// PlayHistory represents a record of played content
type PlayHistory struct {
	ID              int64  `xorm:"pk autoincr 'id'"`
	FileID          string `xorm:"varchar(50) null 'file_id'"`
	Filename        string `xorm:"varchar(250) not null 'filename'"`
	FilePath        string `xorm:"varchar(250) not null 'filepath'"`
	StartedAt       int64  `xorm:"not null 'started_at'"`
	FinishedAt      int64  `xorm:"null 'finished_at'"`
	DurationSeconds int64  `xorm:"null 'duration_seconds'"`
	IsAd            int    `xorm:"not null default 0 'is_ad'"`
	SkipRequested   int    `xorm:"not null default 0 'skip_requested'"`
}

// TableName returns the table name for PlayHistory
func (PlayHistory) TableName() string {
	return "play_history"
}

// MarkAsFinished marks the playback as finished and calculates duration
func (p *PlayHistory) MarkAsFinished() {
	p.FinishedAt = time.Now().Unix()
	p.DurationSeconds = p.FinishedAt - p.StartedAt
}

// MarkAsSkipped marks the playback as skipped
func (p *PlayHistory) MarkAsSkipped() {
	p.SkipRequested = 1
	p.MarkAsFinished()
}
