package webui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeStaticTableViewFixture writes a table whose cells sort differently
// as numbers and as text, name a WAV file that exists and one that does
// not, and one long cell, and returns its path.
func writeStaticTableViewFixture(t *testing.T) string {
	t.Helper()
	wav, err := os.ReadFile(filepath.Join("..", "tableview", "testdata", "media", "pcm16.wav"))
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	write(t, filepath.Join(directory, "audio", "a.wav"), string(wav))
	path := filepath.Join(directory, "data.csv")
	write(t, path, "id,score,audio,note\n1,3.5,audio/a.wav,short\n2,10,audio/gone.wav,"+strings.Repeat("x", 5000)+"\n3,2,audio/a.wav,short\n")
	return path
}

func TestStaticTableViewInBrowser(t *testing.T) {
	output := filepath.Join(t.TempDir(), "snapshot")
	copied, err := GenerateStaticTableView(writeStaticTableViewFixture(t), output)
	if err != nil || copied.Rows != 3 || copied.MediaFiles != 1 || copied.MediaBytes == 0 {
		t.Fatalf("copied = %+v, %v", copied, err)
	}
	// One copy of a.wav, however many rows name it; none of the missing file.
	if copies, _ := filepath.Glob(filepath.Join(output, "media", "*")); len(copies) != 1 {
		t.Fatalf("media copies = %v", copies)
	}
	server := httptest.NewServer(http.FileServer(http.Dir(output)))
	defer server.Close()
	runTableViewPage(t, server.URL+"/", `
  await until('rows', () => cells().length === 3);
  if (JSON.stringify(headings()) !== '["#","id","score","audio","note"]') fail('headings', headings());
  if (!document.querySelector('.tv-static-note') || document.querySelector('.tv-filter-form')) fail('filter bar', document.getElementById('tv-filterbar').innerHTML);
  for (const control of document.querySelectorAll('.tv-toolbar select, .tv-toolbar input')) {
    if (!control.disabled) fail('enabled control', control.outerHTML);
  }
  if (!document.querySelector('.tv-pager').textContent.includes('long cells are shortened')) fail('cut', document.querySelector('.tv-pager').textContent);

  // Sorting reads the snapshot's orders: numbers by value, both ways.
  const sortBy = (name) => [...document.querySelectorAll('.tv-sort')].find(button => button.textContent.startsWith(name)).click();
  const ids = () => cells().map(row => row[1]).join();
  sortBy('score');
  await until('ascending', () => ids() === '3,1,2');
  sortBy('score');
  await until('descending', () => ids() === '2,1,3');
  sortBy('score');
  await until('file order', () => ids() === '1,2,3');

  // Describe covers the whole file.
  document.querySelector('.tv-describe-toggle').click();
  await until('describe', () => document.querySelectorAll('#tv-table thead tr').length > 1);

  // Media link their copies; a missing file says so.
  if (!cells()[0][3].includes('WAV PCM 16-bit')) fail('properties', cells()[0]);
  if (!cells()[1][3].includes('File not found')) fail('missing', cells()[1]);
  const link = document.querySelector('#tv-table tbody a.tv-media-path');
  if (new URL(link.href).pathname !== '/media/0.wav' || (await window.fetch(link.href)).status !== 200) fail('media link', link.href);

  // What only the live server does is refused with the reason.
  const refused = await window.fetch('api/rows?filters=' + encodeURIComponent('[{"column":0,"op":"empty"}]'));
  if (refused.status !== 400 || !(await refused.text()).includes('Filters are not available')) fail('filters');
  const separator = await window.fetch('api/rows?sep=%7C');
  if (separator.status !== 400) fail('separator', separator.status);
`)
}

func TestStaticTableViewRefuses(t *testing.T) {
	path := writeStaticTableViewFixture(t)
	var many strings.Builder
	many.WriteString("n\n")
	for n := 0; n <= staticTableViewRowLimit; n++ {
		fmt.Fprintf(&many, "%d\n", n)
	}
	tooMany := filepath.Join(t.TempDir(), "many.csv")
	write(t, tooMany, many.String())
	for _, test := range []struct{ name, path, output, want string }{
		{"a directory", filepath.Dir(path), t.TempDir(), "is a directory"},
		{"an output holding the file", path, filepath.Dir(path), "which it would replace"},
		{"an output above the file", path, filepath.Dir(filepath.Dir(path)), "which it would replace"},
		{"too many rows", tooMany, filepath.Join(t.TempDir(), "out"), "holds at most 10000"},
		{"a missing file", filepath.Join(t.TempDir(), "missing.csv"), t.TempDir(), "no such file"},
	} {
		_, err := GenerateStaticTableView(test.path, test.output)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%s: err = %v, want %q", test.name, err, test.want)
		}
	}
	// What was refused is still there.
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
