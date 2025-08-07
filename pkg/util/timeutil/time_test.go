package timeutil

import (
	"testing"
	"time"
)

func TestLatest(t *testing.T) {
	time1 := time.Date(2021, time.January, 10, 5, 0, 0, 0, time.UTC)
	time2 := time.Date(2022, time.January, 15, 15, 0, 0, 0, time.UTC)
	time3 := time.Date(2023, time.January, 20, 12, 0, 0, 0, time.UTC)
	latestDate := Latest(time1, time2, time3)
	if !latestDate.Equal(time3) {
		t.Errorf("Date Comparison Failed")
		t.Fail()
	}
}

func TestEarliest(t *testing.T) {
	time1 := time.Date(2021, time.January, 10, 5, 0, 0, 0, time.UTC)
	time2 := time.Date(2022, time.January, 15, 15, 0, 0, 0, time.UTC)
	time3 := time.Date(2023, time.January, 20, 12, 0, 0, 0, time.UTC)
	latestDate := Earliest(false, time1, time2, time3)
	if !latestDate.Equal(time1) {
		t.Errorf("Date Comparison Failed")
		t.Fail()
	}
}
