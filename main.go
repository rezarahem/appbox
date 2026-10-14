// appbox installs AppImages into the current user's application menu.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type paths struct{ data, apps, icons, records, desktop string }
type record struct {
	ID, Name, AppImage, Desktop string
	Icon                        string
}

var commandName = "appbox"

func userPaths() (paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return paths{}, err
	}
	data := filepath.Join(home, ".local", "share", "appbox")
	return paths{data, filepath.Join(data, "apps"), filepath.Join(data, "icons"), filepath.Join(data, "records"), filepath.Join(home, ".local", "share", "applications")}, nil
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func slug(s string) string {
	s = strings.TrimSuffix(strings.TrimSuffix(s, ".AppImage"), ".appimage")
	s = nonAlnum.ReplaceAllString(strings.ToLower(s), "-")
	return strings.Trim(s, "-")
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(to)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func uniqueID(records, base string) string {
	for n, candidate := 2, base; ; n, candidate = n+1, fmt.Sprintf("%s-%d", base, n) {
		if _, err := os.Stat(filepath.Join(records, candidate+".json")); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
}

func findIcon(appimage, id, icons string) (string, error) {
	tmp, err := os.MkdirTemp("", "appbox-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, appimage, "--appimage-extract")
	command.Dir, command.Stdout, command.Stderr = tmp, io.Discard, io.Discard
	if err := command.Run(); err != nil {
		return "", nil
	}
	root := filepath.Join(tmp, "squashfs-root")
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return "", nil
	}
	type icon struct {
		path  string
		score [3]int64
	}
	var candidates []icon
	needle := strings.ReplaceAll(slug(id), "-", "")
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".png" && ext != ".svg" && ext != ".xpm" && ext != ".ico" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		stem := strings.ReplaceAll(slug(strings.TrimSuffix(filepath.Base(path), ext)), "-", "")
		var nameMatch, preferred int64
		if strings.Contains(stem, needle) || strings.Contains(needle, stem) {
			nameMatch = 1
		}
		if ext == ".png" || ext == ".svg" {
			preferred = 1
		}
		candidates = append(candidates, icon{path, [3]int64{nameMatch, preferred, info.Size()}})
		return nil
	})
	if len(candidates) == 0 {
		return "", nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		for n := range candidates[i].score {
			if candidates[i].score[n] != candidates[j].score[n] {
				return candidates[i].score[n] > candidates[j].score[n]
			}
		}
		return false
	})
	target := filepath.Join(icons, id+strings.ToLower(filepath.Ext(candidates[0].path)))
	return target, copyFile(candidates[0].path, target)
}

func desktopExec(path string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "$", `\$`, "`", "\\`").Replace(path) + `"`
}

func refreshDesktopDatabase(desktop string) {
	if bin, err := exec.LookPath("update-desktop-database"); err == nil {
		_ = exec.Command(bin, desktop).Run()
	}
}

func add(sourceArg, customName string) error {
	source, err := filepath.Abs(sourceArg)
	if err != nil {
		return err
	}
	info, err := os.Stat(source)
	if err != nil || info.IsDir() {
		return fmt.Errorf("file not found: %s", sourceArg)
	}
	if !strings.EqualFold(filepath.Ext(source), ".appimage") {
		return errors.New("expected a file ending in .AppImage")
	}
	p, err := userPaths()
	if err != nil {
		return err
	}
	for _, dir := range []string{p.apps, p.icons, p.records, p.desktop} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	displayName := customName
	if displayName == "" {
		displayName = strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return errors.New("app name cannot be empty")
	}
	id := uniqueID(p.records, slug(displayName))
	if id == "" {
		id = uniqueID(p.records, "appimage")
	}
	installed := filepath.Join(p.apps, id+".AppImage")
	if err := copyFile(source, installed); err != nil {
		return err
	}
	if err := os.Chmod(installed, info.Mode()|0111); err != nil {
		return err
	}
	icon, err := findIcon(installed, id, p.icons)
	if err != nil {
		return err
	}
	desktop := filepath.Join(p.desktop, "appbox-"+id+".desktop")
	content := strings.Join([]string{"[Desktop Entry]", "Type=Application", "Name=" + displayName, "Exec=" + desktopExec(installed), "Terminal=false", "Categories=Utility;"}, "\n")
	if icon != "" {
		content += "\nIcon=" + icon
	}
	content += "\n"
	if err := os.WriteFile(desktop, []byte(content), 0644); err != nil {
		return err
	}
	rec := record{id, displayName, installed, desktop, icon}
	bytes, _ := json.Marshal(rec)
	if err := os.WriteFile(filepath.Join(p.records, id+".json"), bytes, 0644); err != nil {
		return err
	}
	refreshDesktopDatabase(p.desktop)
	status := "no icon found"
	if icon != "" {
		status = "icon found"
	}
	fmt.Printf("Added %s (%s). Run: %s rm %s\n", displayName, status, commandName, id)
	return nil
}

func remove(id string) error {
	p, err := userPaths()
	if err != nil {
		return err
	}
	id = slug(id)
	bytes, err := os.ReadFile(filepath.Join(p.records, id+".json"))
	if err != nil {
		return fmt.Errorf("no installed app named %q", id)
	}
	var rec record
	if err := json.Unmarshal(bytes, &rec); err != nil {
		return err
	}
	for _, path := range []string{rec.AppImage, rec.Desktop, rec.Icon} {
		if path != "" {
			_ = os.Remove(path)
		}
	}
	if err := os.Remove(filepath.Join(p.records, id+".json")); err != nil {
		return err
	}
	refreshDesktopDatabase(p.desktop)
	fmt.Printf("Removed %s.\n", rec.Name)
	return nil
}

func list() error {
	p, err := userPaths()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(p.records)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Println("No AppImages installed by appbox.")
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		bytes, err := os.ReadFile(filepath.Join(p.records, entry.Name()))
		if err != nil {
			continue
		}
		var rec record
		if json.Unmarshal(bytes, &rec) == nil {
			fmt.Printf("%s\t%s\n", rec.ID, rec.Name)
		}
	}
	return nil
}

func usage() {
	fmt.Printf(`%s — simple AppImage installer

Usage:
  %s add APPIMAGE [NAME]     Install an AppImage; NAME is optional.
  %s rm NAME                 Remove an installed AppImage.
  %s list                    List installed AppImages.

Example: %s add MyApp.AppImage "My App Name"
`, commandName, commandName, commandName, commandName, commandName)
}

func main() {
	commandName = filepath.Base(os.Args[0])
	if len(os.Args) < 2 {
		usage()
		return
	}
	var err error
	switch os.Args[1] {
	case "add":
		if len(os.Args) < 3 || len(os.Args) > 4 {
			usage()
			return
		}
		name := ""
		if len(os.Args) == 4 {
			name = os.Args[3]
		}
		err = add(os.Args[2], name)
	case "rm":
		if len(os.Args) != 3 {
			usage()
			return
		}
		err = remove(os.Args[2])
	case "list":
		if len(os.Args) != 2 {
			usage()
			return
		}
		err = list()
	case "help", "--help", "-h":
		usage()
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "appbox:", err)
		os.Exit(1)
	}
}
