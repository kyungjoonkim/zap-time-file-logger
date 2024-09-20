package rolling

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type TimeFileLogger struct {
	PrefixFileName     string        // PrefixFileName is the prefix of the log file name. e.g. : tmp/app-err-
	TimeFormat         string        // TimeFormat is the time format for the log file name. e.g., 2006-01-02. Patterns defined in the time package should be used.
	LogRetentionPeriod time.Duration // LogRetentionPeriod is the duration of time to keep log files. e.g., 24 * time.Hour
	MaxSize            int           // MaxSize is the maximum size in megabytes of the log file before it gets rolled. Default is 100 megabytes.

	writeMutex      sync.Mutex // writeMutex is used to lock the file for writing.
	removeMutex     sync.Mutex // removeMutex is used to remove old log files.
	file            *os.File   // file is the current log file.
	currentFullName string     // currentFullName is the current log file name with full path. e.g., tmp/app-err-2024-09-16-0.log
	dir             string     // dir is the directory of the log file.
	currentName     string     // currentName is the current log file name. e.g., app-err-2024-09-16-0.log
	fileIndex       int        // fileIndex is the index of the log file.
	logFileTime     time.Time  // logFileTime is the time of the created log file.
}

const (
	megabyte       = 1024 * 1024         // 1 megabyte
	ext            = ".log"              // log file extension
	defaultMaxSize = 100                 //	100 megabytes
	hyphen         = "-"                 // hyphen
	defaultName    = "time-log" + hyphen //
	indexFormat    = hyphen + "%d"
)

// Log File Status
type status int

// Enum Values of Log File Status
const (
	InitFile        status = iota // InitFile is created new Log File
	NotChangeFile                 // NotChangeFile is not changed Log File
	ChangeDateFile                // ChangeDateFile is changed Date Log File
	ChangeIndexFile               // ChangeIndexFile is changed Index Log File
)

// Write implements the io.Writer interface.  It writes to the current logfile.
// Entry point
func (f *TimeFileLogger) Write(p []byte) (n int, err error) {
	f.writeMutex.Lock()
	defer f.writeMutex.Unlock()

	curTime := time.Now()

	logStatus, err := f.loggerFileStatus(curTime)
	if err != nil {
		return 0, err
	}

	if logStatus != NotChangeFile {
		err = f.updateLogFileInfo(logStatus, curTime)
		if err != nil {
			return 0, err
		}

		go f.removeOldLogFiles()
	}

	return f.file.Write(p)
}

// timeFormat returns the time format for the log file name. if empty, it returns time.DateOnly
func (f *TimeFileLogger) timeFormat() string {
	if f.TimeFormat == "" {
		f.TimeFormat = time.DateOnly
	}
	return f.TimeFormat
}

// prefixFileName is the prefix of the log file name. e.g. : tmp/app-err- if empty, it returns defaultName
func (f *TimeFileLogger) prefixFileName() string {
	if f.PrefixFileName == "" {
		f.PrefixFileName = defaultName
	}

	if f.PrefixFileName[len(f.PrefixFileName)-1:] != hyphen {
		f.PrefixFileName += hyphen
	}
	return f.PrefixFileName
}

// loggerFileStatus returns the status of the log file.
func (f *TimeFileLogger) loggerFileStatus(curTime time.Time) (status, error) {
	//init file
	if f.file == nil {
		return InitFile, nil
	}
	// check time change
	if f.logFileTime.Format(f.timeFormat()) != curTime.Format(f.timeFormat()) {
		return ChangeDateFile, nil
	}

	info, err := f.file.Stat()
	if err != nil {
		return 0, err
	}

	if info.Size() >= f.max() {
		return ChangeIndexFile, nil
	}

	return NotChangeFile, nil
}

// updateLogFileInfo updates the log file information.
func (f *TimeFileLogger) updateLogFileInfo(logStatus status, curTime time.Time) error {
	if logStatus == NotChangeFile {
		return nil
	}

	if logStatus == InitFile {
		f.dir = filepath.Dir(f.prefixFileName())
		f.logFileTime = curTime
	}

	if logStatus == ChangeDateFile {
		f.logFileTime = curTime
		f.fileIndex = 0
	}

	if logStatus == ChangeIndexFile {
		f.fileIndex++
	}

	f.currentName = filepath.Base(f.prefixFileName()) +
		f.logFileTime.Format(f.timeFormat()) +
		fmt.Sprintf(indexFormat, f.fileIndex) +
		ext
	f.currentFullName = filepath.Join(f.dir, string(filepath.Separator), f.currentName)

	_ = f.close()
	if err := os.MkdirAll(f.dir, 0755); err != nil {
		return fmt.Errorf("can't make directories for new logfile: %s", err)
	}

	var err error
	if f.file, err = os.OpenFile(f.currentFullName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err != nil {
		return err
	}

	return nil
}

// max returns the maximum size in bytes of log files before rolling.
func (f *TimeFileLogger) max() int64 {
	if f.MaxSize == 0 {
		return int64(defaultMaxSize * megabyte)
	}
	return int64(f.MaxSize) * int64(megabyte)
}

// removeOldLogFiles removes old log files.
func (f *TimeFileLogger) removeOldLogFiles() {
	f.removeMutex.Lock()
	defer f.removeMutex.Unlock()

	for _, removeFilePath := range f.removeFileNames() {
		if err := os.Remove(removeFilePath); err != nil {
			fmt.Printf("Error removing old log file %v: %v\n", removeFilePath, err)
		}
	}
}

// removeFileNames returns the list of log files to remove  older than the retention time.
func (f *TimeFileLogger) removeFileNames() []string {
	var removeFilePaths []string

	if f.LogRetentionPeriod == 0 || f.logFileTime.IsZero() {
		return removeFilePaths
	}

	retentionTime := f.logFileTime.Add(-f.LogRetentionPeriod)

	logFiles, err := filepath.Glob(filepath.Join(f.prefixFileName() + "*" + ext))
	if err != nil {
		fmt.Printf("Error globbing the path %v: %v\n", f.dir, err)
		return removeFilePaths
	}

	for _, path := range logFiles {
		if path == f.currentFullName {
			continue
		}

		oldFileTime := f.extractTime(path, retentionTime.Location())
		if oldFileTime.IsZero() || !oldFileTime.Before(retentionTime) {
			continue
		}

		removeFilePaths = append(removeFilePaths, path)
	}

	return removeFilePaths
}

// Close implements io.Closer, and closes the current logfile.
func (f *TimeFileLogger) Close() error {
	f.writeMutex.Lock()
	defer f.writeMutex.Unlock()
	return f.close()
}

// close closes the file if it is open.
func (f *TimeFileLogger) close() error {
	if f.file == nil {
		return nil
	}
	err := f.file.Close()
	f.file = nil
	return err
}

// extractTime extracts the time from the log file name.
func (f *TimeFileLogger) extractTime(totalFileName string, location *time.Location) time.Time {
	var stringFormatTime string
	var found bool

	// discard prefix
	if stringFormatTime, found = strings.CutPrefix(totalFileName, f.prefixFileName()); !found {
		return time.Time{}
	}
	// discard suffix
	if stringFormatTime, found = strings.CutSuffix(stringFormatTime, ext); !found {
		return time.Time{}
	}

	// fileName HavCnt >  formatCnt  : extract time
	if len(strings.Split(stringFormatTime, hyphen)) > len(strings.Split(f.timeFormat(), hyphen)) {
		stringFormatTime = stringFormatTime[:strings.LastIndex(stringFormatTime, hyphen)]
	}

	extractTime, err := time.ParseInLocation(f.timeFormat(), stringFormatTime, location)
	if err != nil {
		return time.Time{}
	}

	return extractTime
}
