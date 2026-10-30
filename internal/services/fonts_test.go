//go:build !windows

package services

import (
	"reflect"
	"testing"
)

func TestParseFcList(t *testing.T) {
	out := "Hack\tBold\t100\n" +
		"Hack\tRegular\t100\n" +
		"Hack\tRegular\t100\n" +
		"DejaVu Sans\tBook\t\n" +
		"DejaVu Sans\tBold\t\n" +
		"\tRegular\t\n" +
		".Hidden\tRegular\t\n" +
		"Noto Sans CJK JP\tRegular\t90\n" +
		"adwaita Mono\t\t100\n"

	got := parseFcList(out)
	want := []FontFamily{
		{Name: "adwaita Mono", Styles: []string{"Regular"}, Monospace: true},
		{Name: "DejaVu Sans", Styles: []string{"Book", "Bold"}, Monospace: false},
		{Name: "Hack", Styles: []string{"Bold", "Regular"}, Monospace: true},
		{Name: "Noto Sans CJK JP", Styles: []string{"Regular"}, Monospace: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseFcList:\n got  %+v\n want %+v", got, want)
	}
}
