package rolling

import (
	"cmp"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

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

type DateFileLogger struct {
	PrefixFileName     string        // PrefixFileName is the prefix of the log file name. e.g. : tmp/app-err-
	TimeFormat         string        // TimeFormat is the time format for the log file name. e.g., 2006-01-02. Patterns defined in the time package should be used.
	LogRetentionPeriod time.Duration // LogRetentionPeriod is the duration of time to keep log files. e.g., 24 * time.Hour
	MaxSize            int           // MaxSize is the maximum size in megabytes of the log file before it gets rolled. Default is 100 megabytes.

	writeMutex sync.Mutex // writeMutex is used to lock the file for writing.
	fileInfo   *loggerFileInfo
	fileMutex  sync.Mutex
}

func (logger *DateFileLogger) Write(logData []byte) (bytesWritten int, err error) {
	logger.writeMutex.Lock()
	defer logger.writeMutex.Unlock()

	curTime := time.Now()
	if err = logger.checkInit(curTime); err != nil {
		return 0, fmt.Errorf("initialization error: %w", err)
	}

	logFileStatus, err := logger.fileStatus(curTime, int64(len(logData)))
	if err != nil {
		return 0, fmt.Errorf("file status error: %w", err)
	}

	if logFileStatus != NotChangeFile {
		err = logger.fileInfo.startFileChange(logFileStatus, curTime)
		if err != nil {
			return 0, fmt.Errorf("file change error: %w", err)
		}

		go func() {
			logger.fileMutex.Lock()
			defer logger.fileMutex.Unlock()
			logger.reNameOrRemoveOldFile(curTime)
		}()

	}

	return logger.fileInfo.file.Write(logData)
}

func (logger *DateFileLogger) checkInit(curTime time.Time) error {
	if logger.fileInfo != nil {
		return nil
	}
	return logger.makeLoggerInfo(curTime)
}

func (logger *DateFileLogger) fileStatus(curTime time.Time, logDataSize int64) (status, error) {
	fileInfo := logger.fileInfo

	if fileInfo.isChangeTime(curTime) {
		return ChangeDateFile, nil
	}

	fileSize, err := fileInfo.fileSize()
	if err != nil {
		return 0, err
	}

	if fileSize+logDataSize > logger.max() {
		return ChangeIndexFile, nil
	}

	return NotChangeFile, nil
}

// max returns the maximum size in bytes of log files before rolling.
func (logger *DateFileLogger) max() int64 {
	if logger.MaxSize == 0 {
		return int64(defaultMaxSize * megabyte)
	}
	return int64(logger.MaxSize) * int64(megabyte)
}

func (logger *DateFileLogger) makeLoggerInfo(curTime time.Time) error {
	prefixName := strings.TrimSpace(logger.PrefixFileName)
	dir := filepath.Dir(prefixName)
	filePrefix := filepath.Base(prefixName)
	format, formatedTime := makeValidDateFormat(logger.TimeFormat, curTime)
	fileName := makeFileName(filePrefix, formatedTime)
	fullFileName := makeFullFileName(dir, fileName)
	logFileTime := curTime

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("can't make directories for new logfile: %s", err)
	}

	file, err := openFile(fullFileName)
	if err != nil {
		return err
	}

	logger.fileInfo = &loggerFileInfo{
		dir:          dir,
		filePrefix:   filePrefix,
		timeFormat:   format,
		fileName:     fileName,
		fileIndex:    0,
		fullFileName: fullFileName,
		logFileTime:  logFileTime,
		file:         file,
	}
	return nil
}

func (logger *DateFileLogger) reNameOrRemoveOldFile(curTime time.Time) {

	backupFiles := make(map[string][]*oldLogFileInfo)
	err := filepath.WalkDir(logger.curDir(), func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() || logger.curFileName() == d.Name() {
			return nil
		}

		if isLogFile, fileInfo := logger.oldLogFileInfo(d.Name(), path, curTime.Location()); isLogFile {
			parseFileInfos := backupFiles[fileInfo.fileName]
			parseFileInfos = append(parseFileInfos, fileInfo)
			backupFiles[fileInfo.fileName] = parseFileInfos
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error globbing the path %v: %v\n", logger.fileInfo.dir, err)
		return
	}

	removeFileInfo := make([]*oldLogFileInfo, 0)
	for _, fileInfos := range backupFiles {
		if fileInfos[0].isRemoveLogFile(curTime, logger.LogRetentionPeriod, logger.fileInfo.timeFormat) {
			for _, info := range fileInfos {
				removeFileInfo = append(removeFileInfo, info)
			}
			continue
		}

		reNameFileInfos := make([]*oldLogFileInfo, 0)
		for _, info := range fileInfos {
			if info.tempTime.IsZero() {
				continue
			}
			reNameFileInfos = append(reNameFileInfos, info)
		}

		if len(reNameFileInfos) == 0 {
			continue
		}

		maxIndexFileInfo := slices.MaxFunc(fileInfos, func(a, b *oldLogFileInfo) int {
			return cmp.Compare(a.index, b.index)
		})

		sort.Slice(reNameFileInfos, func(i, j int) bool {
			return reNameFileInfos[i].tempTime.Before(reNameFileInfos[j].tempTime)
		})

		logger.renameTempFiles(reNameFileInfos, maxIndexFileInfo.index)
	}

	removeOldFiles(removeFileInfo)

}

func removeOldFiles(removeFileInfo []*oldLogFileInfo) {
	for _, info := range removeFileInfo {
		if removeErr := os.Remove(info.totalFileName); removeErr != nil {
			fmt.Printf("Error removing old log file %v: %v\n",
				info.totalFileName, removeErr)
		}
	}
}

func (logger *DateFileLogger) renameTempFiles(reNameFileInfos []*oldLogFileInfo, startIndex int) {
	for _, info := range reNameFileInfos {
		startIndex++
		targetFileName := logger.curDir() + info.fileName + hyphen
		err := os.Rename(info.totalFileName, targetFileName+strconv.Itoa(startIndex)+ext)

		for err != nil {
			startIndex++
			err = os.Rename(info.totalFileName, targetFileName+strconv.Itoa(startIndex)+ext)
		}
	}
}

func (logger *DateFileLogger) canRemovalFile() bool {
	return logger.LogRetentionPeriod > 0
}
func (logger *DateFileLogger) curDir() string {
	return logger.fileInfo.dir + string(filepath.Separator)
}

func (logger *DateFileLogger) curFileName() string {
	return logger.fileInfo.fileName
}

func (logger *DateFileLogger) oldLogFileInfo(fileName string, path string, location *time.Location) (bool, *oldLogFileInfo) {
	cutName, find := strings.CutPrefix(fileName, logger.fileInfo.filePrefix)
	if !find {
		return false, nil
	}

	// 0 : timeFormat-index,
	// 1 : ext
	splitFormatNames := strings.Split(cutName, ext)
	if len(splitFormatNames) < 2 {
		return false, nil
	}
	// tempTime
	timeAndIndex := findTimeAndIndex(splitFormatNames[0])
	tempTime := findTempTime(splitFormatNames[1])

	if logger.emptyTimeFormat() {
		if len(timeAndIndex) == 0 {
			return true, &oldLogFileInfo{
				totalFileName: path,
				fileName:      logger.fileInfo.filePrefix,
				tempTime:      tempTime,
			}
		}

		index, err := strconv.ParseInt(timeAndIndex, 10, 32)
		if err != nil {
			return false, nil
		}
		return true, &oldLogFileInfo{
			totalFileName: path,
			fileName:      logger.fileInfo.filePrefix,
			index:         int(index),
			tempTime:      tempTime,
		}
	}

	if timeAndIndex == "" {
		return false, nil
	}

	splitFormat := strings.Split(logger.fileInfo.timeFormat, hyphen)
	splitTargetName := strings.Split(timeAndIndex, hyphen)

	if len(splitFormat) == len(splitTargetName) {
		targetTime, err := time.ParseInLocation(logger.fileInfo.timeFormat, timeAndIndex, location)
		if err != nil {
			return false, nil
		}

		return true, &oldLogFileInfo{
			totalFileName: path,
			fileName:      logger.fileInfo.filePrefix + hyphen + timeAndIndex,
			formatTime:    timeAndIndex,
			fileTime:      targetTime,
			tempTime:      tempTime,
		}
	} else if len(splitFormat) < len(splitTargetName) {
		if len(splitFormat) < len(splitTargetName[:len(splitTargetName)-1]) {
			return false, nil // invalid DateFormat
		}

		formatTime := strings.Join(splitTargetName[:len(splitTargetName)-1], hyphen)
		targetTime, err := time.ParseInLocation(logger.fileInfo.timeFormat, formatTime, location)
		if err != nil {
			return false, nil
		}

		strIndex := strings.Join(splitTargetName[len(splitTargetName)-1:], "")
		index, err := strconv.ParseInt(strIndex, 10, 32)
		if err != nil {
			return false, nil
		}

		return true, &oldLogFileInfo{
			totalFileName: path,
			fileName:      logger.fileInfo.filePrefix + hyphen + formatTime,
			formatTime:    formatTime,
			fileTime:      targetTime,
			index:         int(index),
			tempTime:      tempTime,
		}
	} else {
		return false, nil
	}
}

func (logger *DateFileLogger) emptyTimeFormat() bool {
	return len(logger.fileInfo.timeFormat) == 0
}

func findTempTime(strMillTime string) time.Time {
	if strMillTime == "" {
		return time.Time{}
	}

	milliseconds, err := strconv.ParseInt(strings.TrimPrefix(strMillTime, dot), 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.UnixMilli(milliseconds)
}

// findTimeAndIndex returns the time and index from the log file name. e.g., -2024-09-16-1 -> 2024-09-16-1
func findTimeAndIndex(strTimeAndIndex string) string {
	if strTimeAndIndex == "" {
		return ""
	}

	if strTimeAndIndex[:1] == hyphen {
		return strTimeAndIndex[1:]
	}
	return strTimeAndIndex
}
