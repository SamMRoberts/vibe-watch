package sessions

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type DiscoverOptions struct {
	Root  string
	Days  int
	Limit int
	Now   time.Time
}

func DefaultRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".codex", "sessions")
}

func Discover(opts DiscoverOptions) ([]FileInfo, error) {
	if opts.Root == "" {
		opts.Root = DefaultRoot()
	}
	if opts.Root == "" {
		return nil, errors.New("sessions root is not configured")
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	if _, err := os.Stat(opts.Root); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var cutoff time.Time
	if opts.Days > 0 {
		day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		cutoff = day.AddDate(0, 0, -(opts.Days - 1))
	}

	var files []FileInfo
	years, err := os.ReadDir(opts.Root)
	if err != nil {
		return nil, err
	}
	for _, yearEntry := range years {
		if !yearEntry.IsDir() {
			continue
		}
		year, ok := parseFixedInt(yearEntry.Name(), 4)
		if !ok {
			continue
		}
		yearPath := filepath.Join(opts.Root, yearEntry.Name())
		months, err := os.ReadDir(yearPath)
		if err != nil {
			return nil, err
		}
		for _, monthEntry := range months {
			if !monthEntry.IsDir() {
				continue
			}
			month, ok := parseFixedInt(monthEntry.Name(), 2)
			if !ok || month < 1 || month > 12 {
				continue
			}
			monthPath := filepath.Join(yearPath, monthEntry.Name())
			days, err := os.ReadDir(monthPath)
			if err != nil {
				return nil, err
			}
			for _, dayEntry := range days {
				if !dayEntry.IsDir() {
					continue
				}
				dayNum, ok := parseFixedInt(dayEntry.Name(), 2)
				if !ok {
					continue
				}
				date := time.Date(year, time.Month(month), dayNum, 0, 0, 0, 0, now.Location())
				if date.Year() != year || int(date.Month()) != month || date.Day() != dayNum {
					continue
				}
				if !cutoff.IsZero() && date.Before(cutoff) {
					continue
				}
				dayPath := filepath.Join(monthPath, dayEntry.Name())
				entries, err := os.ReadDir(dayPath)
				if err != nil {
					return nil, err
				}
				for _, entry := range entries {
					if entry.IsDir() || !strings.HasPrefix(entry.Name(), "rollout-") || !strings.HasSuffix(entry.Name(), ".jsonl") {
						continue
					}
					path := filepath.Join(dayPath, entry.Name())
					info, err := entry.Info()
					if err != nil {
						return nil, err
					}
					files = append(files, FileInfo{
						ID:      SessionID(path),
						Path:    path,
						Date:    date,
						ModTime: info.ModTime(),
					})
				}
			}
		}
	}

	sort.Slice(files, func(i, j int) bool {
		if !files[i].Date.Equal(files[j].Date) {
			return files[i].Date.After(files[j].Date)
		}
		if !files[i].ModTime.Equal(files[j].ModTime) {
			return files[i].ModTime.After(files[j].ModTime)
		}
		return files[i].Path > files[j].Path
	})
	if opts.Limit > 0 && len(files) > opts.Limit {
		files = files[:opts.Limit]
	}
	return files, nil
}

func Resolve(root, idOrPath string, opts DiscoverOptions) (string, error) {
	if idOrPath == "" {
		return "", errors.New("session id or path is required")
	}
	if info, err := os.Stat(idOrPath); err == nil && !info.IsDir() {
		return idOrPath, nil
	}
	if abs, err := filepath.Abs(idOrPath); err == nil {
		if info, statErr := os.Stat(abs); statErr == nil && !info.IsDir() {
			return abs, nil
		}
	}
	opts.Root = root
	opts.Limit = 0
	files, err := Discover(opts)
	if err != nil {
		return "", err
	}
	want := strings.TrimSuffix(filepath.Base(idOrPath), ".jsonl")
	for _, file := range files {
		if file.ID == want || filepath.Base(file.Path) == idOrPath {
			return file.Path, nil
		}
	}
	return "", os.ErrNotExist
}

func parseFixedInt(s string, width int) (int, bool) {
	if len(s) != width {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	return n, err == nil
}
