package cmd

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"
)

func renderTemplateFile(srcPath, destPath string, data map[string]string) error {
	content, err := embeddedTemplates.ReadFile(srcPath)
	if err != nil {
		return err
	}
	tmpl, err := template.New(filepath.Base(srcPath)).Parse(string(content))
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}
	outFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer outFile.Close()
	return tmpl.Execute(outFile, data)
}

// 通用执行器
func runCommandInDir(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func runCommandInDirNoOutput(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	return cmd.Run()
}

const (
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	blue   = "\033[34m"
	reset  = "\033[0m"
)

func logFatalf(msg string) {
	log.Fatalf(red + "[ERROR] " + msg + "\n" + reset)
}

func logInfo(msg string) {
	log.Printf(green + "[INFO] " + msg + "\n" + reset)
}

func firstIllegalChar(s string) string {
	s = strings.TrimSpace(s)
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Han, r):
			return string(r) // 汉字非法
		case r >= 'a' && r <= 'z':
			continue
		case r >= 'A' && r <= 'Z':
			continue
		case r >= '0' && r <= '9':
			continue
		case r == '-' || r == '_':
			continue
		default:
			return string(r) // 非法字符，如 emoji、标点
		}
	}
	return "" // 全部合法
}

func isValidDirName(s string) bool {
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Han, r):
			// 不允许汉字
			return false
		case r >= 'a' && r <= 'z':
			continue
		case r >= 'A' && r <= 'Z':
			continue
		case r >= '0' && r <= '9':
			continue
		case r == '-' || r == '_':
			continue
		default:
			return false
		}
	}
	return true
}

func containsChinese(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func containsSpecialChar(s string) bool {
	for _, r := range s {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Han, r)) {
			return true
		}
	}
	return false
}

func WriteFileWithDirs(path string, data []byte) error {
	dir := filepath.Dir(path)

	// 创建目录（如果不存在）
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}
