package jsonduration

import (
	"strconv"
	"testing"
	"time"
)

func TestParseDecimal(t *testing.T) {
	t.Run("sec", func(t *testing.T) {
		tests := []parseTest{
			{"-0", 0, nil},
			{"1.5", 1500 * time.Millisecond, nil},
			{"0.0000000019", time.Nanosecond, nil},
			{"", 0, strconv.ErrSyntax},
			{"-", 0, strconv.ErrSyntax},
			{"+1", 0, strconv.ErrSyntax},
			{"01", 0, strconv.ErrSyntax},
			{"1.", 0, strconv.ErrSyntax},
			{".5", 0, strconv.ErrSyntax},
			{"1e3", 0, strconv.ErrSyntax},
			{"1.5x", 0, strconv.ErrSyntax},
			{"9223372036.854775808", 0, strconv.ErrRange},
			{"-9223372036.854775809", 0, strconv.ErrRange},
			{"18446744074", 0, strconv.ErrRange},
		}

		testParse(t, Sec, tests)
	})

	t.Run("nano", func(t *testing.T) {
		tests := []parseTest{
			{"1.5", 1, nil},
			{"9223372036854775808", 0, strconv.ErrRange},
			{"99999999999999999999999", 0, strconv.ErrRange},
		}

		testParse(t, Nano, tests)
	})
}
