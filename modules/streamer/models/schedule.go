package models

import "time"

// Schedule represents a video file in the playback schedule (endless loop)
type Schedule struct {
	ID               int64  `xorm:"pk autoincr 'id'"`
	FileID           string `xorm:"varchar(50) not null 'file_id'"`
	FilePath         string `xorm:"varchar(250) not null 'filepath'"`
	SchedulePosition int    `xorm:"not null 'schedule_position'"`
	IsCurrent        int    `xorm:"not null default 0 'is_current'"`
	AddedAt          int64  `xorm:"not null 'added_at'"`
}

// TableName sets the table name for XORM
func (Schedule) TableName() string {
	return "schedule"
}

// MarkAsCurrent marks this schedule item as the current one
func (s *Schedule) MarkAsCurrent() {
	s.IsCurrent = 1
}

// UnmarkAsCurrent unmarks this schedule item as current
func (s *Schedule) UnmarkAsCurrent() {
	s.IsCurrent = 0
}

// GetAddedTime returns the added time as a time.Time
func (s *Schedule) GetAddedTime() time.Time {
	return time.Unix(s.AddedAt, 0)
}
