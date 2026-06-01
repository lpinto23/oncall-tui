package model

import "time"

type Incident struct {
	PagerDutyID string
	Summary     string
	StartTime   *time.Time
	EndTime     *time.Time
	Tags        []string
}
