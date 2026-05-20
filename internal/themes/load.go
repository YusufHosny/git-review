package themes

import (
	"embed"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/YusufHosny/git-review/internal/ui"
	"gopkg.in/yaml.v3"
)

//go:embed defaults/*.yaml
var defaultsFS embed.FS

var defaultOrder = []string{
	"dark", "dracula", "catppuccin", "gruvbox",
	"tokyo-night", "rose-pine", "onedark", "solarized", "light",
}

type base16YAML struct {
	Scheme      string `yaml:"scheme"`
	ChromaTheme string `yaml:"chroma_theme"`
	Base00      string `yaml:"base00"`
	Base01      string `yaml:"base01"`
	Base02      string `yaml:"base02"`
	Base03      string `yaml:"base03"`
	Base04      string `yaml:"base04"`
	Base05      string `yaml:"base05"`
	Base06      string `yaml:"base06"`
	Base07      string `yaml:"base07"`
	Base08      string `yaml:"base08"`
	Base09      string `yaml:"base09"`
	Base0A      string `yaml:"base0A"`
	Base0B      string `yaml:"base0B"`
	Base0C      string `yaml:"base0C"`
	Base0D      string `yaml:"base0D"`
	Base0E      string `yaml:"base0E"`
	Base0F      string `yaml:"base0F"`
}

func LoadAll() []ui.Theme {
	dir := ThemesDir()
	exportDefaults(dir)
	if themes := loadDir(dir); len(themes) > 0 {
		return themes
	}
	return LoadEmbedded()
}

func ThemesDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "git-review", "themes")
}

func LoadEmbedded() []ui.Theme {
	entries, _ := defaultsFS.ReadDir("defaults")
	var themes []ui.Theme
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		data, err := defaultsFS.ReadFile("defaults/" + e.Name())
		if err != nil {
			continue
		}
		if t, err := parseBase16(data); err == nil && t.Name != "" {
			themes = append(themes, t)
		}
	}
	sortThemes(themes)
	return themes
}

func exportDefaults(dir string) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	entries, _ := defaultsFS.ReadDir("defaults")
	for _, e := range entries {
		dst := filepath.Join(dir, e.Name())
		if _, err := os.Stat(dst); err == nil {
			continue
		}
		data, _ := defaultsFS.ReadFile("defaults/" + e.Name())
		_ = os.WriteFile(dst, data, 0644)
	}
}

func loadDir(dir string) []ui.Theme {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var themes []ui.Theme
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		if t, err := parseBase16(data); err == nil && t.Name != "" {
			themes = append(themes, t)
		}
	}
	sortThemes(themes)
	return themes
}

func sortThemes(themes []ui.Theme) {
	rank := func(name string) int {
		for i, n := range defaultOrder {
			if n == name {
				return i
			}
		}
		return len(defaultOrder)
	}
	sort.SliceStable(themes, func(i, j int) bool {
		ri, rj := rank(themes[i].Name), rank(themes[j].Name)
		if ri != rj {
			return ri < rj
		}
		return themes[i].Name < themes[j].Name
	})
}

func parseBase16(data []byte) (ui.Theme, error) {
	var f base16YAML
	if err := yaml.Unmarshal(data, &f); err != nil {
		return ui.Theme{}, err
	}
	if f.Scheme == "" || f.Base00 == "" {
		return ui.Theme{}, fmt.Errorf("missing required fields")
	}
	return fromBase16(f), nil
}

func fromBase16(f base16YAML) ui.Theme {
	bg := strings.ToLower(f.Base00)
	hex := func(s string) lipgloss.Color { return lipgloss.Color("#" + strings.ToLower(s)) }

	cursorFg := hex(f.Base06)
	if isLight(bg) {
		cursorFg = hex(f.Base07)
	}

	chroma := f.ChromaTheme
	if chroma == "" {
		chroma = "nord"
	}

	return ui.Theme{
		Name:           strings.ToLower(f.Scheme),
		AddBg:          lipgloss.Color(blendColors(f.Base0B, bg, 0.18)),
		DelBg:          lipgloss.Color(blendColors(f.Base08, bg, 0.18)),
		CursorAddBg:    lipgloss.Color(blendColors(f.Base0B, bg, 0.38)),
		CursorDelBg:    lipgloss.Color(blendColors(f.Base08, bg, 0.38)),
		CursorCtxBg:    hex(f.Base02),
		CursorAddFg:    cursorFg,
		CursorDelFg:    cursorFg,
		CursorCtxFg:    hex(f.Base05),
		GutterAdd:      hex(f.Base0B),
		GutterDel:      hex(f.Base08),
		GutterCtx:      hex(f.Base03),
		StatusApproved: hex(f.Base0B),
		StatusChanged:  hex(f.Base08),
		StatusViewed:   hex(f.Base0A),
		StatusNew:      hex(f.Base04),
		CommentFg:      hex(f.Base0D),
		CommentBg:      hex(f.Base01),
		SearchBg:       hex(f.Base0A),
		SearchFg:       hex(f.Base00),
		BorderNormal:   hex(f.Base02),
		BorderFocused:  hex(f.Base0D),
		TopBarBg:       hex(f.Base01),
		TopBarFg:       hex(f.Base05),
		StatusBarBg:    hex(f.Base01),
		StatusBarFg:    hex(f.Base05),
		DimText:        hex(f.Base03),
		NormalText:     hex(f.Base05),
		BrightText:     hex(f.Base06),
		AccentText:     hex(f.Base0D),
		AccentText2:    hex(f.Base0C),
		StatsAdded:     hex(f.Base0B),
		StatsDeleted:   hex(f.Base08),
		LineNumberFg:   hex(f.Base03),
		ChromaTheme:    chroma,
	}
}

func blendColors(fg, bg string, alpha float64) string {
	fR, fG, fB := parseHex(fg)
	bR, bG, bB := parseHex(bg)
	r := clamp(int(math.Round(float64(bR)*(1-alpha) + float64(fR)*alpha)))
	g := clamp(int(math.Round(float64(bG)*(1-alpha) + float64(fG)*alpha)))
	b := clamp(int(math.Round(float64(bB)*(1-alpha) + float64(fB)*alpha)))
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

func parseHex(h string) (int, int, int) {
	h = strings.ToLower(strings.TrimPrefix(h, "#"))
	if len(h) != 6 {
		return 0, 0, 0
	}
	r, _ := strconv.ParseInt(h[0:2], 16, 64)
	g, _ := strconv.ParseInt(h[2:4], 16, 64)
	b, _ := strconv.ParseInt(h[4:6], 16, 64)
	return int(r), int(g), int(b)
}

func clamp(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

func isLight(hexColor string) bool {
	r, g, b := parseHex(hexColor)
	return 0.299*float64(r)+0.587*float64(g)+0.114*float64(b) > 128
}
