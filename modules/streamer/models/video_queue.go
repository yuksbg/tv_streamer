package models

import (
	"time"
)

// VideoQueue represents a video in the streaming queue
type VideoQueue struct {
	ID            int64  `xorm:"pk autoincr 'id'"`
	FileID        string `xorm:"varchar(50) not null 'file_id'"`
	AddedAt       int64  `xorm:"not null 'added_at'"`
	Played        int    `xorm:"not null default 0 'played'"`
	PlayedAt      int64  `xorm:"null 'played_at'"`
	QueuePosition int    `xorm:"not null default 0 'queue_position'"`
	IsAd          int    `xorm:"not null default 0 'is_ad'"`
}

// TableName returns the table name for VideoQueue
func (VideoQueue) TableName() string {
	return "video_queue"
}

// IsPlayed returns true if the video has been played
func (v *VideoQueue) IsPlayed() bool {
	return v.Played == 1
}

// MarkAsPlayed marks the video as played with current timestamp
func (v *VideoQueue) MarkAsPlayed() {
	v.Played = 1
	v.PlayedAt = time.Now().Unix()
}
