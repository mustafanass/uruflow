/*
 * Copyright (C) 2025 Mustafa Naseer (Mustafa Gaeed)
 *
 * This file is part of Uruflow, an open-source automation tool.
 *
 * Uruflow is a tool designed to streamline and automate Docker-based deployments.
 *
 * Uruflow is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3 of the License.
 *
 * Uruflow is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with Uruflow. If not, see <https://www.gnu.org/licenses/>.
 */

package utils

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

// Logger wraps the standard logger with file and console output
type Logger struct {
	*log.Logger
	logFile *os.File
}

// NewLogger creates a new logger that writes to both console and file
func NewLogger(prefix string) *Logger {
	logDir := os.Getenv("URUFLOW_LOG_DIR")
	if logDir == "" {
		logDir = "./logs"
	}

	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Printf("WARNING: Failed to create log directory %s: %v. Using console only.", logDir, err)
		return &Logger{
			Logger: log.New(os.Stdout, prefix, log.LstdFlags|log.Lshortfile),
		}
	}

	logFileName := fmt.Sprintf("uruflow-%s.log", time.Now().Format("2006-01-02"))
	logPath := filepath.Join(logDir, logFileName)

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("WARNING: Failed to open log file %s: %v. Using console only.", logPath, err)
		return &Logger{
			Logger: log.New(os.Stdout, prefix, log.LstdFlags|log.Lshortfile),
		}
	}

	multiWriter := io.MultiWriter(os.Stdout, logFile)

	return &Logger{
		Logger:  log.New(multiWriter, prefix, log.LstdFlags|log.Lshortfile),
		logFile: logFile,
	}
}

func (l *Logger) Close() error {
	if l.logFile != nil {
		return l.logFile.Close()
	}
	return nil
}

// Info logs an info message
func (l *Logger) Info(format string, v ...interface{}) {
	l.Printf("INFO: "+format, v...)
}

// Success logs a success message
func (l *Logger) Success(format string, v ...interface{}) {
	l.Printf("SUCCESS: "+format, v...)
}

// Warning logs a warning message
func (l *Logger) Warning(format string, v ...interface{}) {
	l.Printf("WARNING: "+format, v...)
}

// Error logs an error message
func (l *Logger) Error(format string, v ...interface{}) {
	l.Printf("ERROR: "+format, v...)
}

// Deploy logs a deployment message
func (l *Logger) Deploy(format string, v ...interface{}) {
	l.Printf("DEPLOYMENT: "+format, v...)
}

// Docker logs a docker-related message
func (l *Logger) Docker(format string, v ...interface{}) {
	l.Printf("DOCKER: "+format, v...)
}

// Git logs a git-related message
func (l *Logger) Git(format string, v ...interface{}) {
	l.Printf("GIT: "+format, v...)
}

// Webhook logs a webhook message
func (l *Logger) Webhook(format string, v ...interface{}) {
	l.Printf("WEBHOOK: "+format, v...)
}

// Worker logs a worker message
func (l *Logger) Worker(format string, v ...interface{}) {
	l.Printf("WORKER: "+format, v...)
}

// Repository logs a repository message
func (l *Logger) Repository(format string, v ...interface{}) {
	l.Printf("REPOSITORY: "+format, v...)
}

// Config logs a configuration message
func (l *Logger) Config(format string, v ...interface{}) {
	l.Printf("CONFIG: "+format, v...)
}

// Network logs a network-related message
func (l *Logger) Network(format string, v ...interface{}) {
	l.Printf("NETWORK: "+format, v...)
}

// Security logs a security-related message
func (l *Logger) Security(format string, v ...interface{}) {
	l.Printf("SECURITY: "+format, v...)
}

// Debug logs a debug message (only in debug mode)
func (l *Logger) Debug(format string, v ...interface{}) {
	// Only log debug messages if debug mode is enabled
	if os.Getenv("DEBUG") == "true" {
		l.Printf("DEBUG: "+format, v...)
	}
}

// Fatal logs a fatal error and exits
func (l *Logger) Fatal(format string, v ...interface{}) {
	l.Printf("FATAL: "+format, v...)
	if l.logFile != nil {
		l.logFile.Close()
	}
	os.Exit(1)
}

// Startup logs a startup message
func (l *Logger) Startup(format string, v ...interface{}) {
	l.Printf("STARTUP: "+format, v...)
}

// Cleanup logs a cleanup message
func (l *Logger) Cleanup(format string, v ...interface{}) {
	l.Printf("CLEANUP: "+format, v...)
}

// Performance logs a performance-related message
func (l *Logger) Performance(format string, v ...interface{}) {
	l.Printf("PERFORMANCE: "+format, v...)
}

// Queue logs a queue-related message
func (l *Logger) Queue(format string, v ...interface{}) {
	l.Printf("QUEUE: "+format, v...)
}
