package framework

import (
	"path"
	"path/filepath"
	"os"
	"strings"
	"time"
	"fmt"
	"io/ioutil"
)

// SelfPath gets compiled executable file absolute path
func SelfPath() string {
	selfPath, _ := filepath.Abs(os.Args[0])
	return selfPath
}

// get absolute filepath, based on built executable file
func RealPath(fp string) (string, error) {
	if path.IsAbs(fp) {
		return fp, nil
	}
	wd, err := os.Getwd()
	return path.Join(wd, fp), err
}

// SelfDir gets compiled executable file directory
func SelfDir() string {
	return filepath.Dir(SelfPath())
}

// get filepath base name
func Basename(fp string) string {
	return path.Base(fp)
}

func BaseWithoutExt(fp string) string {
	basePath := path.Base(fp)
	extIndex := strings.LastIndex(path.Base(fp), path.Ext(fp))
	if extIndex > 0 {
		return basePath[:extIndex]
	}
	return basePath
}

// get filepath dir name
func Dir(fp string) string {
	return path.Dir(fp)
}

func InsureDir(fp string) error {
	if Exists(fp) {
		return nil
	}
	return os.MkdirAll(fp, os.ModePerm)
}

// mkdir dir if not exist
func EnsureDir(fp string) error {
	return os.MkdirAll(fp, os.ModePerm)
}

// ensure the datadir and make sure it's rw-able
func EnsureDirRW(dataDir string) error {
	err := EnsureDir(dataDir)
	if err != nil {
		return err
	}

	checkFile := fmt.Sprintf("%s/rw.%d", dataDir, time.Now().UnixNano())
	fd, err := Create(checkFile)
	if err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("open %s: rw permission denied", dataDir)
		}
		return err
	}
	Close(fd)
	Remove(checkFile)

	return nil
}

// create one file
func Create(name string) (*os.File, error) {
	return os.Create(name)
}

// remove one file
func Remove(name string) error {
	return os.Remove(name)
}

// close fd
func Close(fd *os.File) error {
	return fd.Close()
}

func Ext(fp string) string {
	return path.Ext(fp)
}

// rename file name
func Rename(src string, target string) error {
	return os.Rename(src, target)
}

// IsFile checks whether the path is a file,
// it returns false when it's a directory or does not exist.
func IsFile(fp string) bool {
	f, e := os.Stat(fp)
	if e != nil {
		return false
	}
	return !f.IsDir()
}

// Exist checks whether a file or directory exists.
// It returns false when the file or directory does not exist.
func Exists(fp string) bool {
	_, err := os.Stat(fp)
	return err == nil || os.IsExist(err)
}

func WriteBytes(filePath string, b []byte) (int, error) {
	os.MkdirAll(path.Dir(filePath), os.ModePerm)
	fw, err := os.Create(filePath)
	if err != nil {
		return 0, err
	}
	defer fw.Close()
	return fw.Write(b)
}

func WriteString(filePath string, s string) (int, error) {
	return WriteBytes(filePath, []byte(s))
}

func AppendString(filePath string, s string) (int, error) {
	os.MkdirAll(path.Dir(filePath), os.ModePerm)
	fw, err := os.OpenFile(filePath, os.O_APPEND, os.ModePerm)
	if err != nil {
		return 0, err
	}
	defer fw.Close()
	return fw.WriteString(s)
}

func AppendLine(filePath string, s string) (int, error) {
	os.MkdirAll(path.Dir(filePath), os.ModePerm)
	var fw *os.File
	if !Exists(filePath) {
		fw, _ = os.OpenFile(filePath, os.O_CREATE, os.ModePerm)
	} else {
		fw, _ = os.OpenFile(filePath, os.O_APPEND, os.ModePerm)
	}
	defer fw.Close()
	return fw.WriteString(fmt.Sprintf("%s\n", s))
}

func ReadString(filePath string) string {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return ""
	}
	return string(content)
}
