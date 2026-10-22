//go:build !darwin

package main

import (
	"os"
	"time"
)

// birthTime: macOS 가 아니면 만든 날짜를 알 수 없어서 수정한 날짜를 써요
func birthTime(st os.FileInfo) time.Time { return st.ModTime() }
