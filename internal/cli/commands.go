package cli

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/stub" // TODO remove again
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func nextSeq(matches []string, seqDigits int) (string, error) {
	if seqDigits <= 0 {
		return "", errors.New("Digits must be positive")
	}

	nextSeq := 1
	if len(matches) > 0 {
		fullFilePath := matches[len(matches)-1]
		_, matchSeqStr := filepath.Split(fullFilePath)
		idx := strings.Index(matchSeqStr, "_")
		if idx < 1 { // Using 1 instead of 0 since there should be at least 1 digit
			return "", errors.New("Malformed migration filename: " + fullFilePath)
		}
		matchSeqStr = matchSeqStr[0:idx]
		var err error
		nextSeq, err = strconv.Atoi(matchSeqStr)
		if err != nil {
			return "", err
		}
		nextSeq++
	}
	if nextSeq <= 0 {
		return "", errors.New("Next sequence number must be positive")
	}

	nextSeqStr := strconv.Itoa(nextSeq)
	if len(nextSeqStr) > seqDigits {
		return "", fmt.Errorf("Next sequence number %s too large. At most %d digits are allowed", nextSeqStr, seqDigits)
	}
	padding := seqDigits - len(nextSeqStr)
	if padding > 0 {
		nextSeqStr = strings.Repeat("0", padding) + nextSeqStr
	}
	return nextSeqStr, nil
}

// cleanDir normalizes the provided directory
func cleanDir(dir string) string {
	dir = path.Clean(dir)
	switch dir {
	case ".":
		return ""
	case "/":
		return dir
	default:
		return dir + "/"
	}
}

// createCmd (meant to be called via a CLI command) creates a new migration
func createCmd(dir string, startTime time.Time, format string, name string, ext string, seq bool, seqDigits int) {
	dir = cleanDir(dir)
	if seq && format != defaultTimeFormat {
		log.fatalErr(errors.New("The seq and format options are mutually exclusive"))
	}
	var prefix string
	if seq {
		if seqDigits <= 0 {
			log.fatalErr(errors.New("Digits must be positive"))
		}
		matches, err := filepath.Glob(filepath.Join(dir, "*"+ext))
		if err != nil {
			log.fatalErr(err)
		}
		nextSeqStr, err := nextSeq(matches, seqDigits)
		if err != nil {
			log.fatalErr(err)
		}
		prefix = nextSeqStr
	} else {
		switch format {
		case "":
			log.fatal("Time format may not be empty")
		case "unix":
			prefix = strconv.FormatInt(startTime.Unix(), 10)
		case "unixNano":
			prefix = strconv.FormatInt(startTime.UnixNano(), 10)
		default:
			prefix = startTime.Format(format)
		}
	}

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		log.fatalErr(err)
	}
	up, down := generateMigrationFiles(dir, prefix, name, ext)
	createFile(up)
	createFile(down)
}

func createFile(fname string) {
	file, err := os.Create(fname)
	if err != nil {
		log.fatalErr(err)
		return
	}
	err = file.Close()
	if err != nil {
		log.fatalErr(err)
	}
}

func gotoCmd(m *migrate.Migrate, v uint) {
	if err := m.Migrate(v); err != nil {
		if err != migrate.ErrNoChange {
			log.fatalErr(err)
		} else {
			log.Println(err)
		}
	}
}

func upCmd(m *migrate.Migrate, limit int) {
	if limit >= 0 {
		if err := m.Steps(limit); err != nil {
			if err != migrate.ErrNoChange {
				log.fatalErr(err)
			} else {
				log.Println(err)
			}
		}
	} else {
		if err := m.Up(); err != nil {
			if err != migrate.ErrNoChange {
				log.fatalErr(err)
			} else {
				log.Println(err)
			}
		}
	}
}

func downCmd(m *migrate.Migrate, limit int) {
	if limit >= 0 {
		if err := m.Steps(-limit); err != nil {
			if err != migrate.ErrNoChange {
				log.fatalErr(err)
			} else {
				log.Println(err)
			}
		}
	} else {
		if err := m.Down(); err != nil {
			if err != migrate.ErrNoChange {
				log.fatalErr(err)
			} else {
				log.Println(err)
			}
		}
	}
}

func dropCmd(m *migrate.Migrate) {
	if err := m.Drop(); err != nil {
		log.fatalErr(err)
	}
}

func forceCmd(m *migrate.Migrate, v int) {
	if err := m.Force(v); err != nil {
		log.fatalErr(err)
	}
}

func versionCmd(m *migrate.Migrate) {
	v, dirty, err := m.Version()
	if err != nil {
		log.fatalErr(err)
	}
	if dirty {
		log.Printf("%v (dirty)\n", v)
	} else {
		log.Println(v)
	}
}

// numDownMigrationsFromArgs returns an int for number of migrations to apply
// and a bool indicating if we need a confirm before applying
func numDownMigrationsFromArgs(applyAll bool, args []string) (int, bool, error) {
	if applyAll {
		if len(args) > 0 {
			return 0, false, errors.New("-all cannot be used with other arguments")
		}
		return -1, false, nil
	}

	switch len(args) {
	case 0:
		return -1, true, nil
	case 1:
		downValue := args[0]
		n, err := strconv.ParseUint(downValue, 10, 64)
		if err != nil {
			return 0, false, errors.New("can't read limit argument N")
		}
		return int(n), false, nil
	default:
		return 0, false, errors.New("too many arguments")
	}
}

func generateMigrationFiles(dir, prefix, name, ext string) (up, down string) {
	base := filepath.Join(dir, fmt.Sprintf("%s_%s", prefix, name))
	up = base + ".up" + ext
	down = base + ".down" + ext
	return
}
