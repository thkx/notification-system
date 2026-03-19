package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// LogLevel 日志级别
type LogLevel int

// 定义日志级别常量
const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelFatal
)

// LogFormat 日志格式
type LogFormat int

// 定义日志格式常量
const (
	LogFormatText LogFormat = iota
	LogFormatJSON
)

// Logger 日志结构体
type Logger struct {
	debugLogger *log.Logger
	infoLogger  *log.Logger
	warnLogger  *log.Logger
	errorLogger *log.Logger
	fatalLogger *log.Logger
	level       LogLevel
	format      LogFormat
	output      io.Writer
}

// LoggerConfig 日志配置
type LoggerConfig struct {
	Level  LogLevel  // 日志级别
	Format LogFormat // 日志格式
	Output io.Writer // 输出目标
}

// NewLogger 创建一个新的日志实例
func NewLogger(config *LoggerConfig) *Logger {
	if config == nil {
		config = &LoggerConfig{
			Level:  LogLevelInfo,
			Format: LogFormatText,
			Output: os.Stdout,
		}
	}
	
	// 如果没有指定输出目标，使用标准输出
	if config.Output == nil {
		config.Output = os.Stdout
	}
	
	return &Logger{
		debugLogger: log.New(config.Output, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile),
		infoLogger:  log.New(config.Output, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile),
		warnLogger:  log.New(config.Output, "WARN: ", log.Ldate|log.Ltime|log.Lshortfile),
		errorLogger: log.New(config.Output, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile),
		fatalLogger: log.New(config.Output, "FATAL: ", log.Ldate|log.Ltime|log.Lshortfile),
		level:       config.Level,
		format:      config.Format,
		output:      config.Output,
	}
}

// logEntry 日志条目
type logEntry struct {
	Level     string    `json:"level"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
	File      string    `json:"file"`
	Line      int       `json:"line"`
	Function  string    `json:"function"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// getCallerInfo 获取调用者信息
func getCallerInfo() (file string, line int, function string) {
	pc, file, line, ok := runtime.Caller(3) // 跳过3层调用栈
	if !ok {
		return "unknown", 0, "unknown"
	}
	
	// 提取文件名
	file = filepath.Base(file)
	
	// 提取函数名
	function = runtime.FuncForPC(pc).Name()
	function = function[strings.LastIndex(function, ".")+1:]
	
	return file, line, function
}

// log 通用日志方法
func (l *Logger) log(level LogLevel, format string, v ...interface{}) {
	if level < l.level {
		return
	}
	
	message := fmt.Sprintf(format, v...)
	file, line, function := getCallerInfo()
	
	if l.format == LogFormatJSON {
		// 输出JSON格式日志
		entry := logEntry{
			Level:     level.String(),
			Timestamp: time.Now(),
			Message:   message,
			File:      file,
			Line:      line,
			Function:  function,
		}
		
		jsonData, err := json.Marshal(entry)
		if err == nil {
			fmt.Fprintln(l.output, string(jsonData))
			return
		}
		// 如果JSON序列化失败，回退到文本格式
	}
	
	// 输出文本格式日志
	switch level {
	case LogLevelDebug:
		l.debugLogger.Output(4, message)
	case LogLevelInfo:
		l.infoLogger.Output(4, message)
	case LogLevelWarn:
		l.warnLogger.Output(4, message)
	case LogLevelError:
		l.errorLogger.Output(4, message)
	case LogLevelFatal:
		l.fatalLogger.Output(4, message)
	}
}

// String 实现fmt.Stringer接口
func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "debug"
	case LogLevelInfo:
		return "info"
	case LogLevelWarn:
		return "warn"
	case LogLevelError:
		return "error"
	case LogLevelFatal:
		return "fatal"
	default:
		return "unknown"
	}
}

// Debug 输出Debug级别日志
func (l *Logger) Debug(format string, v ...interface{}) {
	l.log(LogLevelDebug, format, v...)
}

// Info 输出Info级别日志
func (l *Logger) Info(format string, v ...interface{}) {
	l.log(LogLevelInfo, format, v...)
}

// Warn 输出Warn级别日志
func (l *Logger) Warn(format string, v ...interface{}) {
	l.log(LogLevelWarn, format, v...)
}

// Error 输出Error级别日志
func (l *Logger) Error(format string, v ...interface{}) {
	l.log(LogLevelError, format, v...)
}

// Fatal 输出Fatal级别日志并退出程序
func (l *Logger) Fatal(format string, v ...interface{}) {
	l.log(LogLevelFatal, format, v...)
	os.Exit(1)
}

// SetLevel 设置日志级别
func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}

// SetFormat 设置日志格式
func (l *Logger) SetFormat(format LogFormat) {
	l.format = format
}

// 全局日志实例
var globalLogger = NewLogger(nil)

// SetGlobalLogger 设置全局日志实例
func SetGlobalLogger(logger *Logger) {
	globalLogger = logger
}

// GetGlobalLogger 获取全局日志实例
func GetGlobalLogger() *Logger {
	return globalLogger
}

// Debug 全局Debug日志
func Debug(format string, v ...interface{}) {
	globalLogger.Debug(format, v...)
}

// Info 全局Info日志
func Info(format string, v ...interface{}) {
	globalLogger.Info(format, v...)
}

// Warn 全局Warn日志
func Warn(format string, v ...interface{}) {
	globalLogger.Warn(format, v...)
}

// Error 全局Error日志
func Error(format string, v ...interface{}) {
	globalLogger.Error(format, v...)
}

// Fatal 全局Fatal日志
func Fatal(format string, v ...interface{}) {
	globalLogger.Fatal(format, v...)
}

// SetLevel 设置全局日志级别
func SetLevel(level LogLevel) {
	globalLogger.SetLevel(level)
}

// SetFormat 设置全局日志格式
func SetFormat(format LogFormat) {
	globalLogger.SetFormat(format)
}

