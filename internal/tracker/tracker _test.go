package tracker

import "testing"

func TestOutcomeFromPrice(t *testing.T) {
	cases := []struct {
		name  string
		price float64
		dir   string
		want  string
	}{
		{"buy tp", 110, "BUY", "TP1_HIT"},
		{"buy sl", 95, "BUY", "SL_HIT"},
		{"sell tp", 90, "SELL", "TP1_HIT"},
		{"sell sl", 105, "SELL", "SL_HIT"},
		{"pending", 100, "BUY", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := outcomeFromPrice(tc.price, tc.dir, 110, 95); got != tc.want && tc.name != "sell tp" && tc.name != "sell sl" {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			if tc.name == "sell tp" && outcomeFromPrice(tc.price, tc.dir, 90, 105) != tc.want {
				t.Fatalf("sell TP classification failed")
			}
			if tc.name == "sell sl" && outcomeFromPrice(tc.price, tc.dir, 90, 105) != tc.want {
				t.Fatalf("sell SL classification failed")
			}
		})
	}
}
