package webui

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/rotari-dev/rotari/internal/tableview"
)

//go:embed assets/table_view_static.js
var tableViewStaticJS string

// staticTableViewRowLimit bounds the rows a static snapshot holds: every
// row, and its order under each sort, is in the page.
const staticTableViewRowLimit = 10_000

// staticTableViewNote is what a static snapshot's page says it cannot do.
const staticTableViewNote = "This is a static snapshot: filters, the separator, and the header row can be changed only in rotari tableview."

// staticTableView is what a static snapshot embeds, read by
// table_view_static.js.
type staticTableView struct {
	Info    tableViewInfo      `json:"info"`
	Columns []tableview.Column `json:"columns"`
	Notes   []string           `json:"notes,omitempty"`
	Rows    []tableview.Row    `json:"rows"`
	// Cut lists the rows, counted from 0, with a cell cut short.
	Cut []int `json:"cut,omitempty"`
	// Ascending and Descending hold, per column, every row's number,
	// counted from 0, in the order that column sorts them.
	Ascending  [][]int               `json:"ascending"`
	Descending [][]int               `json:"descending"`
	Describe   tableview.Description `json:"describe"`
	// Media maps a cell's media path to its copy, relative to the page.
	Media map[string]string `json:"media"`
}

// StaticTableViewCopy is what a static snapshot holds: the rows, and the
// media files copied with their total size.
type StaticTableViewCopy struct {
	Rows, MediaFiles int
	MediaBytes       int64
}

// GenerateStaticTableView writes a static snapshot of the Table viewer for
// one delimited file to outputDir (rotari tableview --static-dir): the
// file read with the separator its extension suggests and a header row,
// every row with its order under each sort and its Describe, and the media
// files its cells name, copied beside the page. The page is titled with the
// file's name alone. It reads only under the file's directory, as rotari
// tableview does.
func GenerateStaticTableView(path, outputDir string) (StaticTableViewCopy, error) {
	source, err := commandTableViewSource(path)
	if err != nil {
		return StaticTableViewCopy{}, err
	}
	if source.directory {
		return StaticTableViewCopy{}, fmt.Errorf("%s is a directory; a static snapshot takes a delimited file", path)
	}
	output, err := filepath.Abs(outputDir)
	if err != nil {
		return StaticTableViewCopy{}, err
	}
	// The export directory is replaced; it must not hold what it shows.
	if _, _, inside := tableview.RootOf(source.target.Roots[0], []string{output}); inside {
		return StaticTableViewCopy{}, fmt.Errorf("--static-dir %s contains %s, which it would replace", outputDir, path)
	}
	snapshot, copies, err := snapshotTableView(source)
	if err != nil {
		return StaticTableViewCopy{}, err
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return StaticTableViewCopy{}, err
	}
	var escaped bytes.Buffer
	json.HTMLEscape(&escaped, data)
	scripts := "<script>\nwindow.__ROTARI_TABLE_VIEW_STATIC__ = " + escaped.String() + ";\n</script>\n" +
		`    <script src="table_view_static.js"></script>`
	page := tableViewPage([]string{"web_fonts.css", "web_tokens.css"}, scripts)
	if err := os.RemoveAll(outputDir); err != nil {
		return StaticTableViewCopy{}, err
	}
	if err := writeStaticFonts(outputDir); err != nil {
		return StaticTableViewCopy{}, err
	}
	for name, contents := range map[string]string{
		"index.html":           page,
		"table_view.js":        tableViewJS,
		"table_view_panes.js":  tableViewPanesJS,
		"table_view_static.js": tableViewStaticJS,
		"table_view.css":       tableViewCSS,
		"web_fonts.css":        fontFaceCSS("fonts/"),
		"web_tokens.css":       webTokensCSS,
	} {
		if err := writeStaticWebPage(filepath.Join(outputDir, name), contents); err != nil {
			return StaticTableViewCopy{}, err
		}
	}
	copied := StaticTableViewCopy{Rows: len(snapshot.Rows), MediaFiles: len(copies)}
	for copy, media := range copies {
		written, err := copyStaticTableViewMedia(source.target, media, filepath.Join(outputDir, filepath.FromSlash(copy)))
		if err != nil {
			return StaticTableViewCopy{}, err
		}
		copied.MediaBytes += written
	}
	return copied, nil
}

// snapshotTableView reads what a static snapshot embeds, and the media
// files to copy: absolute paths by their copies' paths relative to the
// page.
func snapshotTableView(source tableViewSource) (staticTableView, map[string]string, error) {
	info, err := tableViewInfoOf(source)
	if err != nil {
		return staticTableView{}, nil, err
	}
	// A snapshot is for publishing: it names the file, not where it was.
	info.Name, info.Static = filepath.Base(source.target.Path), staticTableViewNote
	separator, _ := utf8.DecodeRuneInString(info.Separator)
	table, err := tableview.OpenFile(source.target, tableview.Format{Separator: separator, Header: true})
	if err != nil {
		return staticTableView{}, nil, err
	}
	snapshot := staticTableView{Info: info, Rows: []tableview.Row{}, Media: map[string]string{}}
	for offset := 0; ; offset += tableview.MaxLimit {
		page, err := table.Page(tableview.Query{Offset: offset, Limit: tableview.MaxLimit})
		if err != nil {
			return staticTableView{}, nil, err
		}
		if page.Total > staticTableViewRowLimit {
			return staticTableView{}, nil, fmt.Errorf("%s has %d rows; a static snapshot holds at most %d", source.name, page.Total, staticTableViewRowLimit)
		}
		snapshot.Columns, snapshot.Notes = page.Columns, page.Notes
		if page.Cut {
			// A page says only that some cell is cut; a row's own page
			// says whether it is that row.
			for _, row := range page.Rows {
				one, err := table.Page(tableview.Query{Offset: row.Number - 1, Limit: 1})
				if err != nil {
					return staticTableView{}, nil, err
				}
				if one.Cut {
					snapshot.Cut = append(snapshot.Cut, row.Number-1)
				}
			}
		}
		snapshot.Rows = append(snapshot.Rows, page.Rows...)
		if offset+tableview.MaxLimit >= page.Total {
			break
		}
	}
	for column := range snapshot.Columns {
		for _, descending := range []bool{false, true} {
			order, err := table.Order(tableview.Sort{Column: column, Descending: descending})
			if err != nil {
				return staticTableView{}, nil, err
			}
			if descending {
				snapshot.Descending = append(snapshot.Descending, order)
			} else {
				snapshot.Ascending = append(snapshot.Ascending, order)
			}
		}
	}
	if snapshot.Describe, err = table.Describe(nil); err != nil {
		return staticTableView{}, nil, err
	}
	copies := map[string]string{}
	var total int64
	for _, row := range snapshot.Rows {
		for _, media := range row.Media {
			if media.Error != "" || snapshot.Media[media.Ref] != "" {
				continue
			}
			path, err := source.target.ResolveMedia(false, media.Base, media.Ref)
			if err != nil {
				return staticTableView{}, nil, err
			}
			if media.Properties.Size > staticArtifactFileLimit {
				return staticTableView{}, nil, fmt.Errorf("%s is %d bytes; a static snapshot copies media files of at most %d", media.Ref, media.Properties.Size, staticArtifactFileLimit)
			}
			if total += media.Properties.Size; total > staticArtifactTotalLimit {
				return staticTableView{}, nil, fmt.Errorf("the media files %s names exceed the %d bytes a static snapshot copies", source.name, staticArtifactTotalLimit)
			}
			copy := "media/" + strconv.Itoa(len(copies)) + strings.ToLower(filepath.Ext(path))
			copies[copy], snapshot.Media[media.Ref] = path, copy
		}
	}
	return snapshot, copies, nil
}

// copyStaticTableViewMedia copies a media file, opened under the source's
// roots, into a static snapshot, and returns its size.
func copyStaticTableViewMedia(target tableview.Target, path, destination string) (int64, error) {
	file, err := target.OpenMedia(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return 0, err
	}
	out, err := os.Create(destination)
	if err != nil {
		return 0, err
	}
	written, err := io.Copy(out, io.LimitReader(file, staticArtifactFileLimit+1))
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err == nil && written > staticArtifactFileLimit {
		err = fmt.Errorf("%s grew past the %d bytes a static snapshot copies", path, staticArtifactFileLimit)
	}
	return written, err
}
