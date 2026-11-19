package gtk

import (
	"fmt"
	"image/color"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The palettes must match the web GUI's theme token by token.
func TestPalettesMatchGtkCSS(t *testing.T) {
	data, err := os.ReadFile("../../../frontend/src/lib/theme/gtk.css")
	if os.IsNotExist(err) {
		t.Skip("the web GUI's theme is gone")
	}
	if err != nil {
		t.Fatal(err)
	}
	css := string(data)
	block := func(sel string) string {
		i := strings.Index(css, sel)
		if i < 0 {
			t.Fatalf("%q not found in gtk.css", sel)
		}
		j := strings.Index(css[i:], "{") + i
		k := strings.Index(css[j:], "}") + j
		return css[j+1 : k]
	}
	tokenRe := regexp.MustCompile(`--gtk-([a-z0-9-]+):\s*([^;]+);`)
	parse := func(b string) map[string]string {
		out := map[string]string{}
		for _, m := range tokenRe.FindAllStringSubmatch(b, -1) {
			out[m[1]] = strings.TrimSpace(m[2])
		}
		return out
	}
	dark := parse(block(":root,\n[data-theme=\"dark\"]"))
	light := parse(block("[data-theme=\"light\"]"))

	check := func(p Palette, tokens map[string]string, fallback map[string]string) {
		v := reflect.ValueOf(p)
		ty := v.Type()
		seen := 0
		for i := 0; i < ty.NumField(); i++ {
			f := ty.Field(i)
			if f.Type != reflect.TypeOf(color.NRGBA{}) {
				continue
			}
			name := cssName(f.Name)
			raw, ok := tokens[name]
			if !ok {
				raw, ok = fallback[name]
			}
			if !ok {
				t.Errorf("%s: --gtk-%s not in gtk.css", p.Name, name)
				continue
			}
			want, err := parseColor(raw)
			if err != nil {
				t.Errorf("%s: --gtk-%s: %v", p.Name, name, err)
				continue
			}
			if got := v.Field(i).Interface().(color.NRGBA); got != want {
				t.Errorf("%s.%s = %v, gtk.css --gtk-%s is %s", p.Name, f.Name, got, name, raw)
			}
			seen++
		}
		for name, raw := range tokens {
			if _, err := parseColor(raw); err == nil && !hasField(ty, name) {
				t.Errorf("%s: gtk.css color --gtk-%s has no palette field", p.Name, name)
			}
		}
		if seen < 50 {
			t.Errorf("%s: only %d colors checked", p.Name, seen)
		}
	}
	check(Dark, dark, nil)
	check(Light, light, dark)
}

func cssName(field string) string {
	var sb strings.Builder
	for i, r := range field {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				sb.WriteByte('-')
			}
			r += 'a' - 'A'
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

func hasField(ty reflect.Type, css string) bool {
	for i := 0; i < ty.NumField(); i++ {
		if cssName(ty.Field(i).Name) == css {
			return true
		}
	}
	return false
}

var rgbaRe = regexp.MustCompile(`^rgba\(\s*(\d+),\s*(\d+),\s*(\d+),\s*([\d.]+)\s*\)$`)

func parseColor(s string) (color.NRGBA, error) {
	if strings.HasPrefix(s, "#") && len(s) == 7 {
		v, err := strconv.ParseUint(s[1:], 16, 32)
		if err != nil {
			return color.NRGBA{}, err
		}
		return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}, nil
	}
	if m := rgbaRe.FindStringSubmatch(s); m != nil {
		r, _ := strconv.Atoi(m[1])
		g, _ := strconv.Atoi(m[2])
		b, _ := strconv.Atoi(m[3])
		a, _ := strconv.ParseFloat(m[4], 64)
		return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: uint8(a*255 + 0.5)}, nil
	}
	return color.NRGBA{}, fmt.Errorf("not a color: %q", s)
}
