package sessions

import (
	"errors"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ScanOptions struct {
	Root  string
	Since *time.Time
	Until *time.Time
	Limit int
}

type File struct {
	Path    string    `json:"-"`
	ID      string    `json:"id"`
	Date    time.Time `json:"date"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

func Discover(opts ScanOptions) ([]File, error) {
	if opts.Root == "" {
		return nil, errors.New("session root is required")
	}
	var files []File
	err := filepath.WalkDir(opts.Root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".jsonl") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		date, ok := dateFromPath(opts.Root, path)
		if !ok {
			return nil
		}
		if opts.Since != nil && date.Before(*opts.Since) {
			return nil
		}
		if opts.Until != nil && date.After(*opts.Until) {
			return nil
		}
		files = append(files, File{
			Path:    path,
			ID:      strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())),
			Date:    date,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].Date.Equal(files[j].Date) {
			return files[i].ID < files[j].ID
		}
		return files[i].Date.Before(files[j].Date)
	})
	if opts.Limit > 0 && len(files) > opts.Limit {
		files = files[:opts.Limit]
	}
	return files, nil
}

func dateFromPath(root, path string) (time.Time, bool) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return time.Time{}, false
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) < 4 {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.DateOnly, parts[0]+"-"+parts[1]+"-"+parts[2])
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}
