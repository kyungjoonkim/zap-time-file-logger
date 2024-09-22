package rolling

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type loggerFileInfo struct {
	dir             string    // dir is the directory of the log file. e.g., tmp/
	filePrefix      string    // filePrefix is the prefix of the log file name. e.g. : app-err-
	timeFormat      string    // timeFormat is the time format for the log file name. e.g., 2006-01-02. Patterns defined in the time package should be used.
	fullFileName    string    // fullFileName is the current log file name with full path. e.g., tmp/app-err-2024-09-16.log
	fileName        string    // currentName is the current log file name. e.g., app-err-2024-09-16.log
	fileIndex       int       // fileIndex is the index of the log file.
	logFileTime     time.Time // logFileTime is the time of the created log file.
	file            *os.File  // file is the current log file.
	backupFileInfos []*backupFileInfo
}

type backupFileInfo struct {
	backupFileName string
	logTIme        time.Time
}

func (f *loggerFileInfo) isTimeFormat() bool {
	return f.timeFormat != ""
}

func (f *loggerFileInfo) isChangeTime(curTime time.Time) bool {
	return f.isTimeFormat() && f.logFileTime.Format(f.timeFormat) != curTime.Format(f.timeFormat)
}

func (f *loggerFileInfo) fileSize() (int64, error) {
	info, err := f.file.Stat()
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func (f *loggerFileInfo) updateFileInfo(fileStatus status, curTime time.Time) error {
	if fileStatus == ChangeDateFile {
		return f.updateFileNameForDate(curTime)
	}

	if fileStatus == ChangeIndexFile {
		return f.updateFileNameForIndex(curTime)
	}
	return nil
}

func (f *loggerFileInfo) updateFileNameForDate(curTime time.Time) (err error) {
	f.fileName = makeFileName(f.filePrefix, makeFormatTime(curTime, f.timeFormat))
	f.fullFileName = makeFullFileName(f.dir, f.fileName)
	f.logFileTime = curTime
	f.fileIndex = 0
	return f.changeFile(curTime)
}

func (f *loggerFileInfo) updateFileNameForIndex(curTime time.Time) (err error) {
	f.fileIndex++
	return f.changeFile(curTime)
}

func (f *loggerFileInfo) changeFile(curTime time.Time) (err error) {
	if err = f.closeFile(); err != nil {
		return err
	}

	var backupFileName string
	if backupFileName, err = f.reNameFile(curTime); err != nil {
		return err
	}

	f.backupFileInfos = append(f.backupFileInfos, &backupFileInfo{
		backupFileName: backupFileName,
		logTIme:        curTime,
	})
	f.file, err = openFile(f.fileName)
	return err
}

func (f *loggerFileInfo) reNameFile(curTime time.Time) (string, error) {
	tempFileName := f.fullFileName + "." + strconv.FormatInt(curTime.UnixMilli(), 10)
	err := os.Rename(f.fullFileName, tempFileName)
	if err != nil {
		return "", err
	}
	return tempFileName, nil
}

func (f *loggerFileInfo) closeFile() error {
	if f.file == nil {
		return nil
	}
	err := f.file.Close()
	f.file = nil
	return err
}

func validateTimeFormat(timeFormat string) bool {
	targetTime, err := time.Parse(time.Now().Format(timeFormat), timeFormat)
	return err == nil && !targetTime.IsZero()
}

func openFile(fullFileName string) (*os.File, error) {
	if fullFileName == "" {
		return nil, errors.New("file name is empty")
	}
	return os.OpenFile(fullFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
}

func makeFileName(filePrefix, formatedCurTime string) string {
	return filePrefix + formatedCurTime + ext
}

func makeFullFileName(dir, fileName string) string {
	return dir + string(filepath.Separator) + fileName
}

func makeTimeFormat(timeFormat string) string {
	timeFormat = strings.TrimSpace(timeFormat)
	if validateTimeFormat(timeFormat) {
		return timeFormat
	}
	return ""
}

func makeFormatTime(time time.Time, timeFormat string) string {
	if timeFormat == "" {
		return ""
	}
	return time.Format(timeFormat)
}
