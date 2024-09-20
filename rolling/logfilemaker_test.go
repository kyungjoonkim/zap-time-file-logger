package rolling

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"strings"
	"testing"
	"time"
)

var fileLogger = &TimeFileLogger{
	PrefixFileName:     "/tmp/app-err-",
	TimeFormat:         "2006-01-02",
	LogRetentionPeriod: 24 * time.Hour,
}

func TestSomething(t *testing.T) {
	assert.True(t, true, "True is true!")
}

func TestMakeFile(t *testing.T) {
	file := zapcore.AddSync(&TimeFileLogger{
		PrefixFileName: "app-err-",
		TimeFormat:     time.TimeOnly,
	})
	core := zapcore.NewTee(
		zapcore.NewCore(zapcore.NewConsoleEncoder(zap.NewProductionEncoderConfig()), file, zap.InfoLevel),
	)
	logger := zap.New(core)
	defer logger.Sync()
	logger.Info("Hello from Zap!")
}

func TestTimeCheck(t *testing.T) {
	old := time.Now().Add(-fileLogger.LogRetentionPeriod)
	fmt.Println(old.Format(fileLogger.TimeFormat))

	//assert.Equal(t, time.DateOnly, fileLogger.timeFormat())
}

func TestCutPrefix(t *testing.T) {
	after, found := strings.CutPrefix("/tmp/app-err-2024-09-16.log", fileLogger.prefixFileName())
	if !found {
		fmt.Println(after)
		return
	}
	after, found = strings.CutSuffix(after, ext)
	if !found {
		fmt.Println(after)
		return
	}

	fmt.Println(after)
	nowFormat := time.Now().Format(fileLogger.timeFormat())
	fmt.Println(nowFormat)

	// 문자열을 time.Time 타입으로 변환
	parseTime, err := time.Parse(time.DateOnly, nowFormat)
	if err != nil {
		fmt.Println("Error parsing time:", err)
		return
	}

	fmt.Println("Parsed time:", parseTime)

}

func TestTimeDuration(t *testing.T) {
	logger := &TimeFileLogger{
		PrefixFileName:     "log/app-err-",
		TimeFormat:         "2006-01-02",
		LogRetentionPeriod: 24 * time.Hour,
	}

	_ = logger.updateLogFileInfo(InitFile, time.Now())

	names := logger.removeFileNames()
	for _, name := range names {
		fmt.Println(name)
	}

}

func TestLog(t *testing.T) {
	logger := createLogger()
	defer logger.Sync()
	sugar := logger.Sugar()
	//sugar.Infof("Hello from Zap!: %s", time.Now().Format("2006-01-02 15:04:05"))

	processEndTime := time.Now().Add(30 * time.Second)

	goroutineLimit := make(chan struct{}, 10) // 최대 100개의 고루틴만 동시에 실행
	for processEndTime.After(time.Now()) {

		goroutineLimit <- struct{}{} // 고루틴 실행 전 채널에 값 추가
		go func() {
			defer func() {
				<-goroutineLimit
			}() // 고루틴 종료 후 채널에서 값 제거
			sugar.Infof("Hello from Zap!: %s", time.Now().Format("2006-01-02 15:04:05"))
		}()
	}
}

func createLogger() *zap.Logger {
	stdout := zapcore.AddSync(os.Stdout)
	level := zap.NewAtomicLevelAt(zap.InfoLevel)
	developmentCfg := zap.NewDevelopmentEncoderConfig()
	developmentCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	consoleEncoder := zapcore.NewConsoleEncoder(developmentCfg)

	core := zapcore.NewTee(
		zapcore.NewCore(consoleEncoder, stdout, level),
		newCoreFile(),
	)

	return zap.New(core)
}

func newCoreFile() zapcore.Core {
	//file1, _ := os.OpenFile("example.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	file := zapcore.AddSync(&TimeFileLogger{
		PrefixFileName:     "log/app-err-",
		TimeFormat:         time.DateTime,
		LogRetentionPeriod: 5 * time.Second,
	})

	productionCfg := zap.NewProductionEncoderConfig()
	productionCfg.TimeKey = "timestamp"
	productionCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	return zapcore.NewCore(zapcore.NewConsoleEncoder(productionCfg), file, zap.InfoLevel)
}
