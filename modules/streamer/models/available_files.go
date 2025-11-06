package models

// AvailableFiles represents files that are available for streaming
type AvailableFiles struct {
	FileID      string `xorm:"pk varchar(50) 'file_id'"`
	FilePath    string `xorm:"varchar(250) not null 'filepath'"`
	FileSize    int64  `xorm:"not null 'file_size'"`
	VideoLength int64  `xorm:"not null 'video_length'"`
	AddedTime   int64  `xorm:"not null 'added_time'"`
	FFProbeData string `xorm:"text null default '{}' 'ffprobe_data'"`
}

// TableName returns the table name for AvailableFiles
func (AvailableFiles) TableName() string {
	return "availible_files"
}
