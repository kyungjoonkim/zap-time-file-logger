package rolling

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
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
		return 0, err
	}

	logFileStatus, err := logger.fileStatus(curTime, int64(len(logData)))
	if err != nil {
		return 0, err
	}

	if logFileStatus != NotChangeFile {
		err = logger.fileInfo.updateFileInfo(logFileStatus, curTime)
		if err != nil {
			return 0, err
		}
		go func() {
			logger.fileMutex.Lock()
			defer logger.fileMutex.Unlock()
			//todo: save backup file 작업 해야함
			//logger.saveBackupFile(curTime)
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

	timeFormat := makeTimeFormat(logger.TimeFormat)
	formatedCurTime := makeFormatTime(curTime, timeFormat)
	fileName := makeFileName(filePrefix, formatedCurTime)
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
		timeFormat:   timeFormat,
		fileName:     fileName,
		fileIndex:    0,
		fullFileName: fullFileName,
		logFileTime:  logFileTime,
		file:         file,
	}
	return nil
}

//func (logger *DateFileLogger) saveBackupFile(curTime time.Time) {
//	backupFileInfos := logger.fileInfo.backupFileInfos
//	if len(backupFileInfos) == 0 {
//		return
//	}
//
//	sort.Slice(backupFileInfos, func(i, j int) bool {
//		return backupFileInfos[i].logTIme.Before(backupFileInfos[j].logTIme)
//	})
//
//	for _, info := range backupFileInfos {
//		backupPrefix, found := strings.CutSuffix(info.backupFileName, ext)
//		if !found {
//			continue
//		}
//
//		err := logger.processBackupFile(backupPrefix)
//		if err != nil {
//			fmt.Printf("Error globbing the path %v: %v\n", logger.fileInfo.dir, err)
//			continue
//		}
//	}
//
//}
//
//func (logger *DateFileLogger) processBackupFile(backupPrefix string) error {
//	logFiles, err := filepath.Glob(filepath.Join(backupPrefix + "-*" + ext))
//	if err != nil {
//		return err
//	}
//
//	fileCnt := len(logFiles)
//	if fileCnt <= 0 {
//		return nil
//	}
//
//	var lastInfo fs.FileInfo
//	for _, backupFileName := range logFiles {
//		info, err := os.Stat(backupFileName)
//		if err != nil {
//			continue
//		}
//
//		if lastInfo == nil {
//			lastInfo = info
//			continue
//		}
//
//		if info.ModTime().After(lastInfo.ModTime()) {
//			lastInfo = info
//		}
//	}
//
//	if lastInfo == nil {
//		return nil
//	}
//
//	name, found := strings.CutPrefix(lastInfo.Name(), backupPrefix)
//	if !found {
//		return nil
//	}
//
//	strings.
//
//}
