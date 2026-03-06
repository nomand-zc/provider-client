package main

import (
	"os"

	"github.com/nomand-zc/provider-client/cli/cmd"
	"github.com/nomand-zc/provider-client/log"
	"github.com/nomand-zc/provider-client/utils"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func initLogger() {
	logPath := utils.If(os.Getenv("LOG_FILE") != "", os.Getenv("LOG_FILE"), "./app.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("日志初始化失败,err: %s", err)
	}

	// 设置日志级别
	log.SetLevel(log.LevelDebug)

	// 创建多写入器（同时输出到控制台和文件）
	multiWriter := zapcore.NewMultiWriteSyncer(
		zapcore.AddSync(os.Stdout),
		zapcore.AddSync(logFile),
	)

	log.Default = zap.New(
		zapcore.NewCore(
			zapcore.NewConsoleEncoder(log.EncoderConfig),
			multiWriter,
			log.ZapLevel,
		),
		zap.AddCaller(),
		zap.AddCallerSkip(1),
	).Sugar()

	log.ContextDefault = zap.New(
		zapcore.NewCore(
			zapcore.NewConsoleEncoder(log.EncoderConfig),
			multiWriter,
			log.ZapLevel,
		),
		zap.AddCaller(),
		zap.AddCallerSkip(1),
	).Sugar()
}

func init() {
	// 初始化日志配置
	initLogger()
}

func main() {
	cmd.Execute()
}
