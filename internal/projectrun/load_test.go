package projectrun

import (
	"encoding/binary"
	"reflect"
	"runtime"
	"testing"

	"github.com/rotari-dev/rotari/internal/model"
)

func TestParseLoadAverage(t *testing.T) {
	got := parseLoadAverage("0.52 1.25 2.00 3/512 12345\n")
	want := &model.LoadAverage{One: 0.52, Five: 1.25, Fifteen: 2}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLoadAverage = %+v, want %+v", got, want)
	}
	for _, input := range []string{"", "0.52 1.25", "0.52 x 2.00"} {
		if got := parseLoadAverage(input); got != nil {
			t.Fatalf("parseLoadAverage(%q) = %+v, want nil", input, got)
		}
	}
}

func TestDecodeDarwinLoadAverage(t *testing.T) {
	record := make([]byte, 24)
	binary.LittleEndian.PutUint32(record[0:], 1024)  // 0.5 at a scale of 2048
	binary.LittleEndian.PutUint32(record[4:], 2048)  // 1.0
	binary.LittleEndian.PutUint32(record[8:], 5120)  // 2.5
	binary.LittleEndian.PutUint64(record[16:], 2048) // fscale
	got := decodeDarwinLoadAverage(record)
	want := model.LoadAverage{One: 0.5, Five: 1, Fifteen: 2.5}
	if got == nil || *got != want {
		t.Fatalf("decodeDarwinLoadAverage = %+v, want %+v", got, want)
	}
	if got := decodeDarwinLoadAverage(record[:20]); got != nil {
		t.Fatalf("short record decoded as %+v", got)
	}
	zeroScale := append([]byte(nil), record...)
	binary.LittleEndian.PutUint64(zeroScale[16:], 0)
	if got := decodeDarwinLoadAverage(zeroScale); got != nil {
		t.Fatalf("zero scale decoded as %+v", got)
	}
}

// TestHostLoadAverageIsRead checks that the platforms CI runs record a load.
func TestHostLoadAverageIsRead(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skipf("no load average source on %s", runtime.GOOS)
	}
	load := readLoadAverage()
	if load == nil {
		t.Fatal("no load average read on this host")
	}
	if load.One < 0 || load.Five < 0 || load.Fifteen < 0 {
		t.Fatalf("negative load average %+v", load)
	}
}
