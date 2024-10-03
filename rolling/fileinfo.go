package rolling

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	dot = "."
)

type loggerFileInfo struct {
	dir          string    // dir is the directory of the log file. e.g., tmp/
	filePrefix   string    // filePrefix is the prefix of the log file name. e.g. : app-err-
	timeFormat   string    // timeFormat is the time format for the log file name. e.g., 2006-01-02. Patterns defined in the time package should be used.
	fullFileName string    // fullFileName is the current log file name with full path. e.g., tmp/app-err-2024-09-16.log
	fileName     string    // currentName is the current log file name. e.g., app-err-2024-09-16.log
	fileIndex    int       // fileIndex is the index of the log file.
	logFileTime  time.Time // logFileTime is the time of the created log file.
	file         *os.File  // file is the current log file.
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

func (f *loggerFileInfo) startFileChange(fileStatus status, curTime time.Time) error {
	if fileStatus == NotChangeFile {
		return nil
	}

	if fileStatus == ChangeDateFile {
		f.updateFileInfo(curTime)
	}
	return f.changeFile(curTime)
}

func (f *loggerFileInfo) updateFileInfo(curTime time.Time) {
	_, formatedTime := makeValidDateFormat(f.timeFormat, curTime)
	f.fileName = makeFileName(f.filePrefix, formatedTime)
	f.fullFileName = makeFullFileName(f.dir, f.fileName)
	f.logFileTime = curTime
}
func (f *loggerFileInfo) changeFile(curTime time.Time) (err error) {
	oldFileName := f.file.Name()
	if err = f.closeFile(); err != nil {
		return fmt.Errorf("failed to close file: %w", err)
	}

	if err = f.reNameFile(oldFileName, curTime); err != nil {
		return fmt.Errorf("failed to rename file: %w", err)
	}

	f.file, err = openFile(f.fullFileName)
	if err != nil {
		return fmt.Errorf("failed to open new file: %w", err)
	}
	return nil
}

func (f *loggerFileInfo) reNameFile(oldFileName string, curTime time.Time) error {
	tempFileName := oldFileName + dot + strconv.FormatInt(curTime.UnixMilli(), 10)
	err := os.Rename(oldFileName, tempFileName)
	if err != nil {
		return err
	}
	return nil
}

func (f *loggerFileInfo) closeFile() error {
	if f.file == nil {
		return nil
	}
	err := f.file.Close()
	f.file = nil
	return err
}

func openFile(fullFileName string) (*os.File, error) {
	if fullFileName == "" {
		return nil, errors.New("file name is empty")
	}
	return os.OpenFile(fullFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
}

func makeFileName(filePrefix, formatedCurTime string) string {
	if formatedCurTime == "" {
		return filePrefix + ext
	}
	return filePrefix + hyphen + formatedCurTime + ext
}

func makeFullFileName(dir, fileName string) string {
	return dir + string(filepath.Separator) + fileName
}

func makeValidDateFormat(format string, curTime time.Time) (string, string) {
	format = strings.TrimSpace(format)

	if format != "" && strings.ContainsAny(format, "/") {
		return "", ""
	}

	formatedTime := curTime.Format(format)
	if _, err := time.Parse(format, formatedTime); err != nil {
		return "", ""
	}
	return format, formatedTime
}
