package rolling

import "time"

type oldLogFileInfo struct {
	totalFileName string
	fileName      string
	formatTime    string
	fileTime      time.Time
	index         int
	tempTime      time.Time
}

func (p *oldLogFileInfo) isRemoveLogFile(curTime time.Time, retention time.Duration, layout string) bool {
	if retention <= 0 || layout == "" {
		return false
	}

	parTime, err := time.ParseInLocation(layout, curTime.Format(layout), curTime.Location())
	if err != nil {
		return false
	}

	basTime := parTime.Add(-retention)
	return basTime.After(p.fileTime)
}
